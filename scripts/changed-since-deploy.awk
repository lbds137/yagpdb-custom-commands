# Turns `git diff --name-status -M deployed -- <dirs>` into the paste list for
# `make changed-since-deploy`. See Makefile's changed-since-deploy target.
#
# A(dded)/M(odified): print the path.
# R(enamed) at 100% similarity (pure move): print nothing, nothing to paste.
# R(enamed) below 100% (moved AND edited): print the NEW path.
# D(eleted): print a "remove from YAGPDB" note.
BEGIN { FS = "\t" }

$1 ~ /^R/ {
    new = $3
    if (new ~ /\.gohtml$/ && $1 != "R100") print new
    next
}

$1 == "D" {
    path = $2
    if (path ~ /\.gohtml$/) print "deleted (remove from YAGPDB): " path
    next
}

$1 == "A" || $1 == "M" {
    path = $2
    if (path ~ /\.gohtml$/) print path
}
