# release-notes

Turns one release tag into a changelog entry, a commit and a GitHub Release.
`notes` finds the tag released before the partition (`git tag --list 'v*-wiki'
--sort=-v:refname`), reads every commit between the two, and writes at the top
of CHANGELOG.md an entry headed `## <tag> — <date>` whose Added / Changed /
Fixed items each say one thing a person using Memdoor notices. `approve` is
your gate — nothing is committed, pushed or published until you open it.
`commit` commits CHANGELOG.md as `Changelog for <tag>` and pushes origin main.
`release` creates the GitHub Release for the tag on the public repo
guregodevo/memdoor, with the entry's own text as its notes, and stays green,
doing nothing, when that release already exists.

The partition is the version tag, so it is required: run
`/workflow:release-notes v1.512.0-wiki` (or `memdoor workflow run
release-notes --partition v1.512.0-wiki`); approve the gate with `a` in
`/workflow` or `memdoor workflow approve <run> approve`. A tag is released
once: a rerun finds its entry already in CHANGELOG.md and skips `notes`.

It needs the tag to exist locally, CHANGELOG.md to be the only thing you want
in that commit, and `gh` authenticated with access to guregodevo/memdoor. It
leaves the changelog entry, one commit on main, and the release behind.
