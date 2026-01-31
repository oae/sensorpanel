# Changelog

## [1.0.2](https://github.com/oae/sensorpanel/compare/v1.0.1...v1.0.2) (2026-01-31)


### Bug Fixes

* **ci:** use macos-15-intel for x86_64 builds ([f051002](https://github.com/oae/sensorpanel/commit/f0510028d3dd55f6340c38f16e7d58eab9d604bc))

## [1.0.1](https://github.com/oae/sensorpanel/compare/v1.0.0...v1.0.1) (2026-01-31)


### Bug Fixes

* **ci:** use native runners for cross-platform builds ([8df2e64](https://github.com/oae/sensorpanel/commit/8df2e6418efe92aa8fb58b9d9a804484ac6946c7))

## 1.0.0 (2026-01-31)


### ⚠ BREAKING CHANGES

* Python implementation removed

### Features

* add cross-platform service management ([d06dc2d](https://github.com/oae/sensorpanel/commit/d06dc2ddb3294301c8e1c87c3898ec82b761a745))
* add device profiles and theme dev tools ([089d658](https://github.com/oae/sensorpanel/commit/089d65894d04dbead2dd9f9ed113b82a81e193f0))
* **cli:** add prune command for cleanup ([fb4a108](https://github.com/oae/sensorpanel/commit/fb4a108a143b235402458df406e951afdea8ef2f))
* initial Python implementation with NixOS flake ([01e618d](https://github.com/oae/sensorpanel/commit/01e618d9a102d75944eb185566babe433008c815))
* rewrite core in Go with modular architecture ([f57a42f](https://github.com/oae/sensorpanel/commit/f57a42f45cd147db07521921339a803edbe02666))
* **sensors:** add modular provider system ([a082ac6](https://github.com/oae/sensorpanel/commit/a082ac686bc6211f32cdaf558c38e8c75131491d))


### Bug Fixes

* add linux build tag to config tests ([ee21d06](https://github.com/oae/sensorpanel/commit/ee21d06657847ddb326a7f07bf919b4a74d9e5de))
* cross-platform test compatibility ([001716a](https://github.com/oae/sensorpanel/commit/001716a371de165ebe7982136ee6fcd30f5ac78f))
* respect XDG_DATA_HOME on macOS for testing ([e6362f8](https://github.com/oae/sensorpanel/commit/e6362f8eca4fa78d578787e9eb0557b4e23eee5b))
* **service:** use graphical-session.target for Linux autostart ([10077b3](https://github.com/oae/sensorpanel/commit/10077b3d0c471a8ceb7e86d9bd5b99b5048b805a))


### Code Refactoring

* migrate from Python to Go ([1f0263b](https://github.com/oae/sensorpanel/commit/1f0263bc3ab5cec7018bc95e856852a060ad811a))
