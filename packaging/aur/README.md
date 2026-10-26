# AUR packages

Two PKGBUILDs are kept here:

- `dbrix-bin/` — installs the prebuilt release binary (recommended)
- `dbrix/` — builds from the tagged source with Go

## Publishing to the AUR

The AUR requires its own git repository per package. From a machine with an
AUR account and an SSH key registered at <https://aur.archlinux.org>:

```bash
# prebuilt package
git clone ssh://aur@aur.archlinux.org/dbrix-bin.git
cp dbrix-bin/PKGBUILD dbrix-bin/.SRCINFO dbrix-bin/
cd dbrix-bin && git add -A && git commit -m "Initial upload: 0.3.2" && git push
```

Repeat for `dbrix/` if you also want the source package. After the first upload,
update `pkgver`, `sha256sums` (and `sha256sums_x86_64`/`sha256sums_aarch64` for
`dbrix-bin`) and regenerate `.SRCINFO` with `makepkg --printsrcinfo > .SRCINFO`
before pushing each release.

Once published, users install with:

```bash
yay -S dbrix-bin   # or: yay -S dbrix
```
