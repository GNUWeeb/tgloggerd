#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Read-only audit of the content-addressed file store against telegram_files.
#
# The store holds one blob per stored row, at
#     <TG_STORAGE_DIR>/0a/53/3d/97/ed/<sha256>[.<ext>]
# (src/tgloggerd/StoragePath.hpp, blobPath()). This script walks the store,
# reads (id, sha256, file_ext, on_disk) of every row in one streamed SELECT and
# reports:
#
#   orphan          a blob whose sha256 and extension match no row, by age
#                   (mtime) bucket; "ext-mismatch" when a row has that sha256
#                   under another extension (the web would never serve it)
#   unlinked-left   a blob of a row with on_disk = 0 (should have been
#                   unlinked when it was evicted or never stored)
#   missing         a row with on_disk = 1 whose blob is not in the store
#                   (a row with an ext-mismatch blob is listed here too)
#   empty-dir       an empty fan-out directory (only the deepest one of a
#                   chain of empty directories)
#   bad-name        an entry that does not follow the layout (wrong directory
#                   name or depth, a file outside the leaf level, a name that
#                   is not <64 lowercase hex>[.<ext>], a digest that does not
#                   match its directories, a symlink or other special file)
#
# It never deletes or changes anything, in the store or in the database. It
# reads directory entries and their metadata (lstat), never file contents,
# unless --hash-sample asks it to hash a random sample of blobs to check that
# their names match their bytes. The SQL is a single SELECT in a read-only
# transaction, streamed (mysql --quick), so it holds no locks.
#
# The summary goes to stdout. --list-dir DIR writes the full lists, one file
# per category (orphan.tsv, ext-mismatch.tsv, unlinked-left.tsv, missing.tsv,
# empty-dir.txt, bad-name.tsv, hash-mismatch.tsv), so that a later,
# owner-approved step can act on exactly those entries. Paths in the lists are
# relative to the store directory.
#
# Usage
# -----
#
# The daemon keeps writing while the audit runs. Rows are read first, then
# the store is walked: a blob stored after the SELECT shows up as an orphan in
# the "after snapshot" bucket and is not an orphan; a row evicted during the
# walk may show up as missing. Run it twice and act only on entries in both,
# or on orphans older than a day.
#
# 1. With a mysql client on the machine, credentials from the deployment's
#    .env (TG_DB_HOST, TG_DB_PORT, TG_DB_USER, TG_DB_PASSWORD, TG_DB_NAME).
#    The password is handed to the client through an option file on an
#    inherited pipe, never on a command line, and is never printed:
#
#        scripts/audit_store.py --env-file .env --store data/storage/files \
#            --list-dir ~/audit-store-$(date +%Y%m%d)
#
#    TG_DB_HOST "mysql" resolves only inside the compose network; on the host
#    pass --db-host with the database container's address. --db-host,
#    --db-port, --db-user and --db-name override the .env values; the password
#    comes only from the environment (TG_DB_PASSWORD) or --env-file.
#
# 2. Without a mysql client on the host (the tgloggerd image has neither
#    python3 nor mysql), let the database container run the SELECT with its
#    own credentials (MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE from the same
#    .env), save the rows to a file and audit against it. MYSQL_PWD keeps the
#    password off the command line inside the container; the rows file holds
#    only ids, digests, extensions and flags:
#
#        D=~/audit-store-$(date +%Y%m%d); mkdir -p "$D"
#        T=$(date +%s)
#        docker exec tgloggerd-mysql sh -c \
#            'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -u"$MYSQL_USER" \
#             -D"$MYSQL_DATABASE" --quick --batch --skip-column-names \
#             -e "$0"' \
#            "$(scripts/audit_store.py --print-sql)" > "$D/rows.tsv" &&
#        scripts/audit_store.py --rows "$D/rows.tsv" --snapshot-time "$T" \
#            --store data/storage/files --list-dir "$D/lists"
#
#    Save to a file rather than pipe: with a pipe, a SELECT that fails half
#    way leaves a short row list the script cannot tell from a full one, and
#    every blob it misses is reported as an orphan. --rows - reads stdin all
#    the same. Without --snapshot-time the snapshot is the rows file's mtime,
#    which is when the SELECT ended, not when it began. An empty row list is
#    refused unless --allow-empty-rows is given.
#
# Run it from the checkout as the user that owns the store; it needs read and
# search permission on the store's directories only. --pause-every N
# --pause-secs S sleeps S seconds after every N directory entries to spread
# the metadata IO. It keeps every row in memory, about 400 bytes each (300,000
# rows and 100,000 blobs took 120 MiB and 5 s in a test).
#
# Exit status: 0 when the audit ran (whatever it found), 2 on usage or
# database errors.

import argparse
import os
import random
import re
import subprocess
import sys
import time

SQL = ("SET SESSION TRANSACTION READ ONLY; "
       "SELECT id, LOWER(HEX(sha256)), COALESCE(file_ext, ''), on_disk "
       "FROM telegram_files")

HEX2 = re.compile(r"^[0-9a-f]{2}$")
BLOB = re.compile(r"^([0-9a-f]{64})(?:\.([^./]+))?$")

DAY = 86400
# Upper bounds of the age buckets, in seconds before the snapshot.
BUCKETS = [
    ("< 1 day", DAY),
    ("1-7 days", 7 * DAY),
    ("7-30 days", 30 * DAY),
    ("30-90 days", 90 * DAY),
    ("90-365 days", 365 * DAY),
    (">= 1 year", None),
]
AFTER = "after snapshot"


def die(msg):
    print("audit_store: " + msg, file=sys.stderr)
    sys.exit(2)


def read_env_file(path):
    """KEY=VALUE lines as in .env: '#' starts a comment, values are trimmed."""
    env = {}
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, v = line.split("=", 1)
                v = v.strip()
                if v[:1] in ("'", '"') and v[-1:] == v[:1] and len(v) > 1:
                    v = v[1:-1]
                else:
                    v = re.sub(r"\s+#.*$", "", v).strip()
                env[k.strip()] = v
    except OSError as e:
        die("cannot read %s: %s" % (path, e.strerror))
    return env


def unescape(s):
    """Undo mysql --batch escaping (\\t, \\n, \\0, \\\\)."""
    if "\\" not in s:
        return s
    return re.sub(r"\\(.)", lambda m: {"t": "\t", "n": "\n", "0": "\0"}
                  .get(m.group(1), m.group(1)), s)


def run_mysql(args, cfg):
    """Start the mysql client; the credentials go through an inherited pipe."""
    r, w = os.pipe()
    opts = "[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\n" % (
        cfg["host"], cfg["port"], cfg["user"], cfg["password"])
    cmd = [args.mysql, "--defaults-extra-file=/dev/fd/%d" % r,
           "--quick", "--batch", "--skip-column-names",
           "-D", cfg["name"], "-e", SQL]
    try:
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, pass_fds=(r,),
                             text=True, encoding="utf-8",
                             errors="surrogateescape")
    except OSError as e:
        die("cannot run %s: %s" % (args.mysql, e.strerror))
    os.close(r)
    # The option file is far smaller than a pipe buffer.
    os.write(w, opts.encode("utf-8"))
    os.close(w)
    return p


def load_rows(args):
    """Return (rows, snapshot, stats); rows maps sha -> (id, ext, on_disk)."""
    proc = None
    if args.rows == "-":
        sys.stdin.reconfigure(encoding="utf-8", errors="surrogateescape")
        src = sys.stdin
        snap = time.time()
    elif args.rows:
        try:
            src = open(args.rows, encoding="utf-8",
                       errors="surrogateescape")
            snap = os.fstat(src.fileno()).st_mtime
        except OSError as e:
            die("cannot read %s: %s" % (args.rows, e.strerror))
    else:
        env = dict(os.environ)
        if args.env_file:
            env.update(read_env_file(args.env_file))
        def pick(opt, var, default=""):
            return opt or env.get(var, "").strip() or default
        cfg = {
            "host": pick(args.db_host, "TG_DB_HOST", "127.0.0.1"),
            "port": pick(args.db_port, "TG_DB_PORT", "3306"),
            "user": pick(args.db_user, "TG_DB_USER"),
            "password": pick(None, "TG_DB_PASSWORD"),
            "name": pick(args.db_name, "TG_DB_NAME", "tgloggerd"),
        }
        if not cfg["user"]:
            die("no database user: set TG_DB_USER, --env-file or --db-user")
        snap = time.time()
        proc = run_mysql(args, cfg)
        src = proc.stdout
    if args.snapshot_time is not None:
        snap = args.snapshot_time

    rows = {}
    st = {"rows": 0, "no_sha": 0, "no_sha_on_disk": 0, "on_disk": 0,
          "bad_rows": 0}
    exts = {}
    for line in src:
        line = line.rstrip("\n")
        if not line:
            continue
        f = line.split("\t")
        if len(f) != 4:
            st["bad_rows"] += 1
            continue
        st["rows"] += 1
        rid, sha, ext, on_disk = f
        on = on_disk == "1"
        if on:
            st["on_disk"] += 1
        if sha == "NULL" or not sha:
            st["no_sha"] += 1
            if on:
                st["no_sha_on_disk"] += 1
            continue
        try:
            key = bytes.fromhex(sha)
        except ValueError:
            st["bad_rows"] += 1
            continue
        ext = unescape(ext)
        ext = exts.setdefault(ext, ext)
        rows[key] = (int(rid), ext, on)
    if proc is not None:
        proc.stdout.close()
        if proc.wait() != 0:
            die("mysql failed (exit %d)" % proc.returncode)
    elif src is not sys.stdin:
        src.close()
    if st["bad_rows"]:
        die("%d malformed row line(s); expected id, hex sha256, ext, on_disk"
            " separated by tabs" % st["bad_rows"])
    if not st["rows"] and not args.allow_empty_rows:
        die("no rows read; refusing to call every blob an orphan"
            " (--allow-empty-rows to override)")
    return rows, snap, st


def bucket_of(mtime, snap):
    if mtime > snap:
        return AFTER
    age = snap - mtime
    for name, upper in BUCKETS:
        if upper is None or age < upper:
            return name
    return BUCKETS[-1][0]


class Audit:
    def __init__(self, store, rows, snap, args):
        self.store = store
        self.rows = rows
        self.snap = snap
        self.args = args
        self.seen = set()  # sha of on_disk = 1 rows whose blob was found
        self.blobs = 0
        self.blob_bytes = 0
        self.entries = 0
        self.ok_blobs = []  # paths of blobs matching a row, for --hash-sample
        self.lists = {k: [] for k in ("orphan", "ext-mismatch",
                                      "unlinked-left", "missing",
                                      "empty-dir", "bad-name",
                                      "hash-mismatch")}
        self.orphan_buckets = {}

    def rel(self, path):
        return os.path.relpath(path, self.store)

    def tick(self):
        self.entries += 1
        a = self.args
        if a.pause_every and self.entries % a.pause_every == 0:
            time.sleep(a.pause_secs)

    def bad(self, path, reason, size=0):
        self.lists["bad-name"].append((self.rel(path), size, reason))

    def walk(self, path, prefix):
        """prefix: the hex digits the directories down to path spell."""
        depth = len(prefix) // 2
        try:
            it = os.scandir(path)
        except OSError as e:
            self.bad(path, "unreadable: %s" % e.strerror)
            return
        empty = True
        with it:
            for e in it:
                empty = False
                self.tick()
                self.entry(e, prefix, depth)
        if empty and depth > 0:
            self.lists["empty-dir"].append(self.rel(path))

    def entry(self, e, prefix, depth):
        try:
            st = e.stat(follow_symlinks=False)
        except OSError as err:
            self.bad(e.path, "lstat failed: %s" % err.strerror)
            return
        if e.is_symlink():
            self.bad(e.path, "symlink", st.st_size)
            return
        if e.is_dir(follow_symlinks=False):
            if depth < 5 and HEX2.match(e.name):
                self.walk(e.path, prefix + e.name)
            else:
                why = ("directory at the leaf level" if depth >= 5
                       else "directory name is not two lowercase hex digits")
                self.bad_tree(e.path, why)
            return
        if not e.is_file(follow_symlinks=False):
            self.bad(e.path, "not a regular file", st.st_size)
            return
        if depth < 5:
            self.bad(e.path, "file above the leaf level", st.st_size)
            return
        m = BLOB.match(e.name)
        if not m:
            self.bad(e.path, "name is not <sha256>[.<ext>]", st.st_size)
            return
        sha, ext = m.group(1), m.group(2) or ""
        if not sha.startswith(prefix):
            self.bad(e.path, "digest does not match its directories",
                     st.st_size)
            return
        if ext != ext.lower():
            self.bad(e.path, "extension is not lowercase", st.st_size)
            return
        self.blob(e.path, sha, ext, st)

    def bad_tree(self, path, why):
        """List each file under a misplaced directory, to act on one by one."""
        found = False
        for root, dirs, files in os.walk(path):
            for name in dirs + files:
                self.tick()
                p = os.path.join(root, name)
                try:
                    st = os.lstat(p)
                except OSError:
                    continue
                if not os.path.isdir(p) or os.path.islink(p):
                    found = True
                    self.bad(p, "under a bad directory: " + why,
                             st.st_size)
            if not dirs and not files:
                found = True
                self.bad(root, "empty bad directory: " + why)
        if not found:
            self.bad(path, why)

    def blob(self, path, sha, ext, st):
        self.blobs += 1
        self.blob_bytes += st.st_size
        key = bytes.fromhex(sha)
        row = self.rows.get(key)
        rel = self.rel(path)
        if row is None:
            b = bucket_of(st.st_mtime, self.snap)
            n, s = self.orphan_buckets.get(b, (0, 0))
            self.orphan_buckets[b] = (n + 1, s + st.st_size)
            self.lists["orphan"].append((rel, st.st_size, int(st.st_mtime), b))
            return
        rid, rext, on = row
        if rext != ext:
            self.lists["ext-mismatch"].append(
                (rel, st.st_size, int(st.st_mtime), rid, rext))
            return
        if not on:
            self.lists["unlinked-left"].append(
                (rel, st.st_size, int(st.st_mtime), rid))
            return
        self.seen.add(key)
        if self.args.hash_sample > 0:
            self.ok_blobs.append(path)

    def finish(self):
        for key, (rid, ext, on) in self.rows.items():
            if on and key not in self.seen:
                sha = key.hex()
                parts = [sha[i * 2:i * 2 + 2] for i in range(5)]
                name = sha + ("." + ext if ext else "")
                self.lists["missing"].append(
                    (os.path.join(*parts, name), rid))
        self.lists["missing"].sort(key=lambda r: r[1])

    def hash_sample(self, n):
        import hashlib
        sample = random.sample(self.ok_blobs, min(n, len(self.ok_blobs)))
        for path in sample:
            h = hashlib.sha256()
            try:
                with open(path, "rb") as f:
                    for chunk in iter(lambda: f.read(1 << 20), b""):
                        h.update(chunk)
            except OSError as e:
                self.lists["hash-mismatch"].append(
                    (self.rel(path), "unreadable: " + e.strerror))
                continue
            want = os.path.basename(path)[:64]
            if h.hexdigest() != want:
                self.lists["hash-mismatch"].append(
                    (self.rel(path), h.hexdigest()))
        return len(sample)


def human(n):
    for unit in ("B", "KiB", "MiB", "GiB", "TiB"):
        if n < 1024 or unit == "TiB":
            return ("%d %s" % (n, unit) if unit == "B"
                    else "%.1f %s" % (n, unit))
        n /= 1024.0


def size_of(entries, idx):
    return sum(e[idx] for e in entries)


def tsv(x):
    """A list field, with backslash, tab and newline escaped as mysql does."""
    return (str(x).replace("\\", "\\\\").replace("\t", "\\t")
            .replace("\n", "\\n"))


def write_lists(d, a):
    os.makedirs(d, exist_ok=True)
    heads = {
        "orphan.tsv": ("path\tbytes\tmtime\tage", a.lists["orphan"]),
        "ext-mismatch.tsv": ("path\tbytes\tmtime\trow_id\trow_ext",
                             a.lists["ext-mismatch"]),
        "unlinked-left.tsv": ("path\tbytes\tmtime\trow_id",
                              a.lists["unlinked-left"]),
        "missing.tsv": ("path\trow_id", a.lists["missing"]),
        "empty-dir.txt": (None, a.lists["empty-dir"]),
        "bad-name.tsv": ("path\tbytes\treason", a.lists["bad-name"]),
        "hash-mismatch.tsv": ("path\tactual_sha256",
                              a.lists["hash-mismatch"]),
    }
    for name, (head, items) in heads.items():
        with open(os.path.join(d, name), "w", encoding="utf-8",
                  errors="surrogateescape") as f:
            if head:
                f.write(head + "\n")
            for it in items:
                if not isinstance(it, tuple):
                    it = (it,)
                f.write("\t".join(tsv(x) for x in it) + "\n")


def main():
    ap = argparse.ArgumentParser(
        description="Read-only audit of the file store against"
                    " telegram_files.")
    ap.add_argument("--store", help="the store directory (TG_STORAGE_DIR)")
    ap.add_argument("--rows", metavar="FILE",
                    help="read the rows from FILE ('-' for stdin) instead of"
                    " running mysql; see --print-sql")
    ap.add_argument("--print-sql", action="store_true",
                    help="print the SELECT the rows must come from and exit")
    ap.add_argument("--env-file", help="read TG_DB_* from this .env file")
    ap.add_argument("--db-host")
    ap.add_argument("--db-port")
    ap.add_argument("--db-user")
    ap.add_argument("--db-name")
    ap.add_argument("--mysql", default="mysql", help="the mysql client")
    ap.add_argument("--snapshot-time", type=float, metavar="EPOCH",
                    help="when the rows were read (default: now, or the"
                    " --rows file's mtime)")
    ap.add_argument("--allow-empty-rows", action="store_true")
    ap.add_argument("--list-dir", metavar="DIR",
                    help="write the full lists into DIR")
    ap.add_argument("--hash-sample", type=int, default=0, metavar="N",
                    help="hash N random matching blobs and check their names")
    ap.add_argument("--pause-every", type=int, default=0, metavar="N")
    ap.add_argument("--pause-secs", type=float, default=0.1, metavar="S")
    args = ap.parse_args()

    if args.print_sql:
        print(SQL)
        return 0
    if not args.store:
        die("--store is required")
    store = os.path.abspath(args.store)
    if not os.path.isdir(store):
        die("not a directory: %s" % args.store)
    if args.list_dir:
        ld = os.path.abspath(args.list_dir)
        if ld == store or ld.startswith(store + os.sep):
            die("--list-dir must be outside the store")

    rows, snap, rst = load_rows(args)
    t0 = time.time()
    a = Audit(store, rows, snap, args)
    a.walk(store, "")
    a.finish()
    hashed = a.hash_sample(args.hash_sample) if args.hash_sample > 0 else 0
    took = time.time() - t0

    L = a.lists
    print("store            %s" % store)
    print("snapshot         %s" % time.strftime("%Y-%m-%dT%H:%M:%S%z",
                                                time.localtime(snap)))
    print("rows             %d (%d on_disk = 1, %d without sha256, %d of"
          " those on_disk = 1)" % (rst["rows"], rst["on_disk"],
                                   rst["no_sha"], rst["no_sha_on_disk"]))
    print("store entries    %d, %d blobs, %s, walked in %.1f s"
          % (a.entries, a.blobs, human(a.blob_bytes), took))
    print()
    print("orphan blobs     %d, %s" % (len(L["orphan"]),
                                       human(size_of(L["orphan"], 1))))
    for name, _ in BUCKETS + [(AFTER, None)]:
        if name in a.orphan_buckets:
            n, s = a.orphan_buckets[name]
            print("  %-14s %d, %s" % (name, n, human(s)))
    print("ext mismatch     %d, %s" % (len(L["ext-mismatch"]),
                                       human(size_of(L["ext-mismatch"], 1))))
    print("on_disk = 0 left %d, %s" % (len(L["unlinked-left"]),
                                       human(size_of(L["unlinked-left"], 1))))
    print("missing blobs    %d (rows with on_disk = 1)" % len(L["missing"]))
    print("empty dirs       %d" % len(L["empty-dir"]))
    print("bad names        %d, %s" % (len(L["bad-name"]),
                                       human(size_of(L["bad-name"], 1))))
    if args.hash_sample > 0:
        print("hash sample      %d hashed, %d mismatched"
              % (hashed, len(L["hash-mismatch"])))
    if args.list_dir:
        write_lists(args.list_dir, a)
        print()
        print("lists written to %s" % os.path.abspath(args.list_dir))
    return 0


if __name__ == "__main__":
    sys.exit(main())
