# Gomod2nix
The nix package depends on ./gomod2nix.toml being up to date
to ensure all dependencies are available to nix.

There is a github workflow to update the file on pushes to main.
This means that after any push to main there will be a new commit
within a couple minutes to update the gomod2nix.toml.

The file can also be updated locally if nix is installed by running
```bash
nix develop
gomod2nix
```
