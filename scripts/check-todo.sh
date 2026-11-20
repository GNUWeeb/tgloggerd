#!/bin/bash
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Checks that docs/TODO.md and docs/todo/*.md agree (docs/TODO.md,
# "Checking the list"). Prints every problem and exits non-zero if
# there is any; prints nothing when all is well. Run from the
# repository root.
set -u

problems=0
problem() {
	echo "check-todo: $*"
	problems=$((problems + 1))
}

# Slugs of the rows, in order.
mapfile -t rows < <(grep -oE '^- \[`[a-z0-9-]+`\]' docs/TODO.md |
	sed -E 's/^- \[`([a-z0-9-]+)`\]$/\1/')

# Slugs are unique.
printf '%s\n' "${rows[@]}" | sort | uniq -d | while read -r dup; do
	[ -n "$dup" ] && echo "check-todo: $dup has more than one row"
done | grep . && problems=$((problems + 1))

# Every row has its file and every file its row.
while read -r link; do
	[ -f "docs/$link" ] || problem "docs/TODO.md links to missing docs/$link"
done < <(grep -oE '\(todo/[a-z0-9-]+\.md\)' docs/TODO.md | tr -d '()')
for f in docs/todo/*.md; do
	[ -e "$f" ] || continue
	slug=$(basename "$f" .md)
	grep -qF "(todo/$slug.md)" docs/TODO.md ||
		problem "$f has no row in docs/TODO.md"
done

# Each row's slug matches its link.
grep -E '^- \[`' docs/TODO.md |
	sed -E 's/^- \[`([^`]+)`\]\(todo\/([^)]+)\.md\).*/\1 \2/' |
	while read -r slug link; do
		[ "$slug" = "$link" ] ||
			echo "check-todo: row $slug links to todo/$link.md"
	done | grep . && problems=$((problems + 1))

# Each file starts with its own slug, and every item it lists under
# "Depends on" has a row earlier in docs/TODO.md.
for f in docs/todo/*.md; do
	[ -e "$f" ] || continue
	slug=$(basename "$f" .md)
	[ "$(head -n1 "$f")" = "# $slug" ] ||
		problem "$f does not start with '# $slug'"

	pos=-1
	for i in "${!rows[@]}"; do
		[ "${rows[$i]}" = "$slug" ] && pos=$i
	done
	while read -r dep; do
		dpos=-1
		for i in "${!rows[@]}"; do
			[ "${rows[$i]}" = "$dep" ] && dpos=$i
		done
		if [ "$dpos" -lt 0 ]; then
			problem "$slug depends on $dep, which is not in docs/TODO.md"
		elif [ "$dpos" -ge "$pos" ]; then
			problem "$slug depends on $dep, which is listed after it"
		fi
	done < <(awk '/^## Depends on/{p=1; next} /^## /{p=0} p' "$f" |
		grep -oE '`[a-z0-9-]+`' | tr -d '`')
done

# The list and the design it points into are plain ASCII. (Other
# documentation in the tree predates this rule and is not checked.)
if grep -rnP '[^\x00-\x7F]' docs/TODO.md docs/todo/ docs/design/ 2>/dev/null; then
	problem "non-ASCII characters in the TODO list or design (above)"
fi

exit $((problems > 0))
