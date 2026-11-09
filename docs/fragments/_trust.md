> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.

# What a binary's checks prove

**A binary is only as trustworthy as the place it came from.**
Massimo runs as a service on your network and controls your heating,
so take it from the project's [releases](https://github.com/metaworx/massimo/releases),
its apt repository or its Homebrew tap, and never from a copy elsewhere:
a mirror, a forum post or a prepared image may carry a changed binary,
and no check of the project's can tell.
`SHA256SUMS` proves that a download arrived as the release has it,
not who built it:
it sits on the same page as the binaries,
and whoever could replace one could replace the other.
Once a binary runs,
`massimo update` checks every later release against the release key compiled into it,
and apt against the repository's key,
so what is left to trust is the first install.
To trust no build but your own, build it from the signed source:
[From source](https://github.com/metaworx/massimo#from-source).
