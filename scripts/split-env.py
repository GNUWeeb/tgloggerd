#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Split one env file into the per-secret files under env/ that
# docker-compose.yml gives to each service (env/README).
#
# Every variable goes to exactly one file: a secret to the file of its
# credential (SECRETS below), the compose file's own settings stay in
# .env (COMPOSE), and everything else goes to env/settings.env, which
# every service gets. Comments travel with the variable below them;
# the section headers go to settings.env (and .env) only.
#
# Usage:
#   scripts/split-env.py            split .env into env/*.env (once, when
#                                   moving an existing deployment); .env
#                                   is kept as .env.before-split and
#                                   rewritten with the COMPOSE keys only
#   scripts/split-env.py --example  regenerate env/*.env.example and
#                                   .env.example from a combined example
#                                   given on stdin (maintainers only)
#   scripts/split-env.py --check    check docker-compose.yml against
#                                   SERVICES; exit 1 on a mismatch
#
# Values are copied byte for byte and never printed. Existing env/*.env
# files are never overwritten, and a split deployment is never split
# again.

import os
import re
import stat
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# The secret files and the variables each holds.
SECRETS = {
    "mysql-root": ["MYSQL_ROOT_PASSWORD"],
    "mysql-init": ["MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD"],
    "logger-db": ["TG_DB_PASSWORD"],
    "web-db-ro": ["WEB_DB_RO_PASSWORD"],
    "web-db-app": ["WEB_DB_APP_PASSWORD"],
    "app-key": ["WEB_APP_KEY"],
    "telegram-api": ["TG_API_ID", "TG_API_HASH"],
    "smtp": ["WEBAPI_SMTP_USER", "WEBAPI_SMTP_PASSWORD"],
}

# Read by docker-compose.yml itself (interpolation), not by a service.
COMPOSE = ["TG_UID", "TG_GID", "RENDER_WORKERS"]

# The env files each service gets, in docker-compose.yml's order. The
# daemon needs the MySQL root password for the web table grants it
# applies after its migrations (run.sh, apply_web_grants); mysql needs
# the web passwords for its first-initialization hook.
SERVICES = {
    "mysql": ["settings", "mysql-root", "mysql-init", "web-db-ro", "web-db-app"],
    "tgloggerd": ["settings", "mysql-root", "logger-db", "app-key", "telegram-api"],
    "web": ["settings", "web-db-ro", "web-db-app", "app-key"],
    "discordd": ["settings", "logger-db", "telegram-api"],
    "webapi": ["settings", "web-db-ro", "web-db-app", "app-key", "smtp"],
}

# A name that looks like a secret must be listed in SECRETS: the split
# refuses to put it into settings.env, which every service reads.
SECRET_NAME = re.compile(r"(PASSWORD|SECRET|TOKEN|_KEY|_HASH)$")

INTRO = {
    "settings": "Every setting that is not a secret. Every service gets this file.",
    "mysql-root": "The MySQL root password: the mysql service, and the daemon for the\n# web table grants it applies after its migrations (run.sh).",
    "mysql-init": "The daemon's database and user, created by the mysql image on its\n# first start (and only then).",
    "logger-db": "The daemon's database password: tgloggerd and discordd.",
    "web-db-ro": "The web's read-only database password: web, webapi, and mysql's\n# first-initialization hook.",
    "web-db-app": "The web's own database password: web, webapi, and mysql's\n# first-initialization hook.",
    "app-key": "The key that signs sessions and file links: tgloggerd, web, webapi.",
    "telegram-api": "The Telegram API credentials: tgloggerd and discordd.",
    "smtp": "The SMTP login for mail sent by webapi: webapi only.",
}

ASSIGN = re.compile(r"^\s*([A-Z][A-Z0-9_]*)=")


def dest_of(name):
    for f, names in SECRETS.items():
        if name in names:
            return f
    if name in COMPOSE:
        return ".env"
    if SECRET_NAME.search(name):
        sys.exit(f"split-env: {name} looks like a secret but is in no secret file; "
                 "add it to SECRETS in scripts/split-env.py")
    return "settings"


def split(lines):
    """Return {dest: [lines]} for the lines of one env file."""
    out = {}
    header = []        # the current section header's lines
    header_done = set()  # dests that already have it
    pending = []       # comment lines waiting for the next variable
    in_header = False
    for line in lines:
        s = line.rstrip("\n")
        if s.startswith("# ----"):
            if not in_header:
                header, header_done, pending = [s], set(), []
                in_header = True
            else:
                header.append(s)
                in_header = False
            continue
        if in_header:
            header.append(s)
            continue
        m = ASSIGN.match(s)
        if m:
            d = dest_of(m.group(1))
            buf = out.setdefault(d, [])
            # The secret files are short: their intro says what they
            # are, so only the settings get the section headers.
            if header and d not in header_done and d not in SECRETS:
                if buf:
                    buf.append("")
                buf.extend(header)
                header_done.add(d)
            buf.extend(pending)
            buf.append(s)
            pending = []
        elif s.strip() == "":
            pending = []
        elif s.startswith("#"):
            pending.append(s)
    return out


def write(path, intro, body, mode):
    with open(path, "w") as f:
        os.fchmod(f.fileno(), mode)
        if intro:
            f.write("# " + intro + "\n\n")
        f.write("\n".join(body).strip("\n") + "\n")


def check():
    """docker-compose.yml's env_file lists must be SERVICES."""
    text = open(os.path.join(ROOT, "docker-compose.yml")).read()
    bad = 0
    for svc, files in SERVICES.items():
        m = re.search(r"^  " + svc + r":\n(.*?)(?=^  \S|\Z)", text, re.M | re.S)
        if not m:
            print(f"check: no service {svc}")
            bad += 1
            continue
        got = re.findall(r"^\s+- env/([a-z-]+)\.env$", m.group(1), re.M)
        if got != files:
            print(f"check: {svc} has {got}, want {files}")
            bad += 1
    return bad


def main():
    args = sys.argv[1:]
    if args == ["--check"]:
        sys.exit(1 if check() else 0)
    envdir = os.path.join(ROOT, "env")
    if args == ["--example"]:
        parts = split(sys.stdin.readlines())
        for d, body in parts.items():
            if d == ".env":
                continue
            write(os.path.join(envdir, d + ".env.example"), INTRO[d], body, 0o644)
        return
    if args:
        sys.exit("usage: split-env.py [--example | --check]")

    src = os.path.join(ROOT, ".env")
    backup = src + ".before-split"
    # Once only: a split .env holds the compose settings alone, so a
    # second run would replace the backup of the original with it.
    done = [f for f in sorted(os.listdir(envdir)) if f.endswith(".env")] \
        if os.path.isdir(envdir) else []
    if done or os.path.exists(backup):
        sys.exit("split-env: already split (" + ", ".join(["env/" + f for f in done] +
                 ([".env.before-split"] if os.path.exists(backup) else [])) +
                 " exist); not changing anything")
    parts = split(open(src).readlines())
    os.makedirs(envdir, exist_ok=True)
    # Every file docker-compose.yml names is written, an empty one too
    # (smtp.env when mail is off): compose refuses a missing env file.
    for d in ["settings", *SECRETS]:
        write(os.path.join(envdir, d + ".env"), INTRO[d], parts.get(d, []), 0o600)
    os.replace(src, backup)
    os.chmod(backup, stat.S_IRUSR | stat.S_IWUSR)
    write(src, "docker-compose.yml's own settings; the services' settings and\n"
          "# secrets are in env/ (env/README).", parts.get(".env", []), 0o600)
    names = {d: len([l for l in b if ASSIGN.match(l)]) for d, b in parts.items()}
    for d in sorted(names):
        print(f"{d:16s} {names[d]} variable(s)")
    print(f"old .env kept as {os.path.relpath(backup, ROOT)}")


if __name__ == "__main__":
    main()
