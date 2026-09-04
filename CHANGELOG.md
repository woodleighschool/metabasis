# Changelog

## [3.1.3](https://github.com/woodleighschool/metabasis/compare/3.1.2...3.1.3) (2026-09-04)


### Continuous Integration

* **github-action:** update action jdx/mise-action (v4.2.5 → v4.3.0) ([#120](https://github.com/woodleighschool/metabasis/issues/120)) ([2e56828](https://github.com/woodleighschool/metabasis/commit/2e56828ed3f30b680f7fd215207a15511275f193))


### Miscellaneous Chores

* fresh mise lock ([cac0c59](https://github.com/woodleighschool/metabasis/commit/cac0c59be3454ff7166364a564a73a1a0ed75730))
* **mise:** update tool lefthook (2.1.11 → 2.1.12) ([#124](https://github.com/woodleighschool/metabasis/issues/124)) ([8eb1abc](https://github.com/woodleighschool/metabasis/commit/8eb1abc395e9d4dc1d07ccc4922d8684d1a245f4))
* **mise:** update tool oxfmt (0.65.0 → 0.66.0) ([#128](https://github.com/woodleighschool/metabasis/issues/128)) ([0c24dcc](https://github.com/woodleighschool/metabasis/commit/0c24dcc84f542a1f994f7ebcb265d18027c1d6aa))
* **mise:** update tool zizmor (1.29.0 → 1.30.0) ([#127](https://github.com/woodleighschool/metabasis/issues/127)) ([5de358b](https://github.com/woodleighschool/metabasis/commit/5de358be4ba730183cd4fb16affefaba8aa543a4))

## [3.1.2](https://github.com/woodleighschool/metabasis/compare/3.1.1...3.1.2) (2026-08-28)


### Documentation

* clarify usage and releases ([eac3ac2](https://github.com/woodleighschool/metabasis/commit/eac3ac28dd3c80110b9ab1ee21b793fc7c923785))


### Continuous Integration

* **github-action:** update action docker/github-builder (v1.16.0 → v1.17.0) ([#117](https://github.com/woodleighschool/metabasis/issues/117)) ([e4e6211](https://github.com/woodleighschool/metabasis/commit/e4e6211d79bfabf779428c6e98ed5e24f71c151f))


### Miscellaneous Chores

* **mise:** update tool oxfmt (0.64.0 → 0.65.0) ([#119](https://github.com/woodleighschool/metabasis/issues/119)) ([1815ad8](https://github.com/woodleighschool/metabasis/commit/1815ad8caf210e60347edc57ecd3d472842e5e57))

## [3.1.1](https://github.com/woodleighschool/metabasis/compare/3.1.0...3.1.1) (2026-08-23)


### Bug Fixes

* align daemon runtime logging ([c8851f6](https://github.com/woodleighschool/metabasis/commit/c8851f6a46d0cd3ccc05bb65c146af7aac0b4b92))


### Code Refactoring

* align runtime configuration ([d817fc3](https://github.com/woodleighschool/metabasis/commit/d817fc3e5cd2f9eef45a513da0f0b20021a3e207))


### Documentation

* add repository agent guidance ([15b7bb7](https://github.com/woodleighschool/metabasis/commit/15b7bb7337924102c4eb2a006f8cd655d2f4dc2f))


### Miscellaneous Chores

* align ignore rules ([7c757cc](https://github.com/woodleighschool/metabasis/commit/7c757cc8d6cfd09c3337edc6306fb26033b0da7a))
* align repository conventions ([22d1a8c](https://github.com/woodleighschool/metabasis/commit/22d1a8c1946c2e998ff777b7ac37e1d1ccda154d))
* **release-please:** sync configuration ([654ed32](https://github.com/woodleighschool/metabasis/commit/654ed328846018f0f3476daf2d2e90ad48c82099))

## [3.1.0](https://github.com/woodleighschool/metabasis/compare/3.0.0...3.1.0) (2026-08-21)


### Features

* export metrics and serialize subject writes ([40d7e04](https://github.com/woodleighschool/metabasis/commit/40d7e04789f09554ceb372b69d3e7d1dc088b555))


### Bug Fixes

* preserve unasserted group memberships ([c8718b6](https://github.com/woodleighschool/metabasis/commit/c8718b6ee707dfa07405039a9d7810a374d8c113))
* **reconcile:** remove deadcode ([8d43168](https://github.com/woodleighschool/metabasis/commit/8d43168755d595cadfb56e787b471cfb7ecf6f0e))

## [3.0.0](https://github.com/woodleighschool/ADOverseas/compare/2.5.2...3.0.0) (2026-08-21)


### ⚠ BREAKING CHANGES

* Metabasis v3 replaces the binary, module, config, database schema, webhook contract, and container image.

### Features

* **container:** update image golang (1.26.6 → 1.27.0) ([#102](https://github.com/woodleighschool/ADOverseas/issues/102)) ([8bf7ecc](https://github.com/woodleighschool/ADOverseas/commit/8bf7ecc019dfcf9c626a9e7c55b60fd51726a242))
* **go:** update module github.com/azure/azure-sdk-for-go/sdk/azcore (v1.22.0 → v1.23.0) ([#111](https://github.com/woodleighschool/ADOverseas/issues/111)) ([012813a](https://github.com/woodleighschool/ADOverseas/commit/012813adc5ede7e0846398a84b55d142758bedf3))
* **go:** update module github.com/microsoftgraph/msgraph-sdk-go (v1.100.0 → v1.101.0) ([#90](https://github.com/woodleighschool/ADOverseas/issues/90)) ([b9a983f](https://github.com/woodleighschool/ADOverseas/commit/b9a983f6e09bd1b673df305d25180a6d1ef39a1c))
* **go:** update module google.golang.org/grpc (v1.80.0 → v1.82.1) [security] ([#100](https://github.com/woodleighschool/ADOverseas/issues/100)) ([8802231](https://github.com/woodleighschool/ADOverseas/commit/8802231ccb414c13f002e12e148e54229816ed2c))
* **npm:** update dependency @types/node (26.1.2 → 26.2.0) ([#91](https://github.com/woodleighschool/ADOverseas/issues/91)) ([5a05e39](https://github.com/woodleighschool/ADOverseas/commit/5a05e39f24dde36f149253311803b6c8c6920e7a))
* **npm:** update dependency pnpm (11.21.0 → 11.22.0) ([#88](https://github.com/woodleighschool/ADOverseas/issues/88)) ([ad51b31](https://github.com/woodleighschool/ADOverseas/commit/ad51b31b158fc9d2d743fd3706c05c1e2dd4a216))
* **npm:** update dependency react-hook-form (7.83.0 → 7.85.0) ([#64](https://github.com/woodleighschool/ADOverseas/issues/64)) ([b7fc98e](https://github.com/woodleighschool/ADOverseas/commit/b7fc98ee596dfa76d267a1f34e32b3fa4d5146f7))
* **npm:** update dependency vite (8.1.5 → 8.2.1) ([#85](https://github.com/woodleighschool/ADOverseas/issues/85)) ([c37eabf](https://github.com/woodleighschool/ADOverseas/commit/c37eabf83b6b332c7a36684b0448ce7faa1c1ac3))
* **npm:** update material-ui monorepo ([#63](https://github.com/woodleighschool/ADOverseas/issues/63)) ([089d567](https://github.com/woodleighschool/ADOverseas/commit/089d567f5f05f9486c2a4342249c524008833273))
* rewrite ADOverseas as Metabasis ([#110](https://github.com/woodleighschool/ADOverseas/issues/110)) ([8ddb91d](https://github.com/woodleighschool/ADOverseas/commit/8ddb91dbfef45cdf9970065b697d933ad454642c))


### Bug Fixes

* **ci:** tidy Go module ([fc08e75](https://github.com/woodleighschool/ADOverseas/commit/fc08e75b885b02b4c1a8dbceaebd368afd9eb573))
* **compose:** keep postgres internal ([b9ff6af](https://github.com/woodleighschool/ADOverseas/commit/b9ff6af914cf965a71313ab93a5d73ba2fd7b63a))
* **go:** update module github.com/go-chi/chi/v5 (v5.3.1 → v5.3.2) ([#108](https://github.com/woodleighschool/ADOverseas/issues/108)) ([c1c0c59](https://github.com/woodleighschool/ADOverseas/commit/c1c0c593dc8c03412dc2b1649a22e1765ecf0423))
* **hooks:** skip pnpm lockfile formatting ([1f73d5a](https://github.com/woodleighschool/ADOverseas/commit/1f73d5ac052519dffedb5d52669113ba6005ca1e))
* **lefthook:** allow ignored formatter inputs ([a4d9e99](https://github.com/woodleighschool/ADOverseas/commit/a4d9e990d87c66962625963e40f7407d17b73b5c))
* **npm:** update dependency dayjs (1.11.21 → 1.11.22) ([#98](https://github.com/woodleighschool/ADOverseas/issues/98)) ([9f779d1](https://github.com/woodleighschool/ADOverseas/commit/9f779d1af53e2078e38332c15d4106f1687d6c19))
* **npm:** update dependency dayjs (1.11.22 → 1.11.23) ([#101](https://github.com/woodleighschool/ADOverseas/issues/101)) ([ecf7d30](https://github.com/woodleighschool/ADOverseas/commit/ecf7d30fcbb63f61640928f87269af6ea79def21))
* **npm:** update dependency react-error-boundary (6.1.2 → 6.1.3) ([#95](https://github.com/woodleighschool/ADOverseas/issues/95)) ([4ef84d9](https://github.com/woodleighschool/ADOverseas/commit/4ef84d921bb82bfdbc163b43a3af6779ee66769f))
* **npm:** update react monorepo ([#72](https://github.com/woodleighschool/ADOverseas/issues/72)) ([6a5a873](https://github.com/woodleighschool/ADOverseas/commit/6a5a87328430f20f891c92f69f41e28554175071))
* **pnpm:** regenerate mature lockfile ([cd062ad](https://github.com/woodleighschool/ADOverseas/commit/cd062ad3569ee6db6712cc7e78594b1ae95baadd))
* **tooling:** group toolchain updates ([1132430](https://github.com/woodleighschool/ADOverseas/commit/1132430882b0f4b80ed48c729fc33ae90ec9e284))

## [2.5.2](https://github.com/woodleighschool/ADOverseas/compare/2.5.1...2.5.2) (2026-08-11)


### Bug Fixes

* **graph:** changed the group membership behaviour so it doesn't run into pagination issues ([c0f6cbb](https://github.com/woodleighschool/ADOverseas/commit/c0f6cbbb9e8a02cd92d1e643a658cd60a609cc6c))

## [2.5.1](https://github.com/woodleighschool/ADOverseas/compare/2.5.0...2.5.1) (2026-08-10)


### Bug Fixes

* **ci:** disable automatic mise installs ([465682e](https://github.com/woodleighschool/ADOverseas/commit/465682ec7621d2f3fb6d6cfde2dbdc7628f2114b))
* **deps:** update dependency @vitejs/plugin-react (6.0.4 → 6.0.5) ([#83](https://github.com/woodleighschool/ADOverseas/issues/83)) ([3d456c8](https://github.com/woodleighschool/ADOverseas/commit/3d456c8a51fe5c3ea6239f4bd9f27920ebcea78a))
* **renovate:** wait for complete toolchain groups ([79fb7e2](https://github.com/woodleighschool/ADOverseas/commit/79fb7e258954bbfde76cdda6b6ab45bfe7414e97))

## [2.5.0](https://github.com/woodleighschool/ADOverseas/compare/2.4.6...2.5.0) (2026-07-30)


### Features

* **deps:** update dependency @tanstack/react-query (5.100.14 → 5.101.2) ([#37](https://github.com/woodleighschool/ADOverseas/issues/37)) ([e4499da](https://github.com/woodleighschool/ADOverseas/commit/e4499dacbb771b7dcea375e874207cea436d0d60))
* **deps:** update dependency date-fns (4.3.0 → 4.4.0) ([#38](https://github.com/woodleighschool/ADOverseas/issues/38)) ([b34ac00](https://github.com/woodleighschool/ADOverseas/commit/b34ac00842c81aa36cf286a6cef6e709d10ebf57))
* **deps:** update dependency eslint (10.4.0 → 10.7.0) ([#39](https://github.com/woodleighschool/ADOverseas/issues/39)) ([beff261](https://github.com/woodleighschool/ADOverseas/commit/beff261a4245325e39c61304fdf6c9d780bf20bb))
* **deps:** update dependency favicons (7.2.0 → 7.3.0) ([#40](https://github.com/woodleighschool/ADOverseas/issues/40)) ([143dcea](https://github.com/woodleighschool/ADOverseas/commit/143dcea59c19866ae051488a856534d11920cb17))
* **deps:** update dependency fuse.js (7.3.0 → 7.5.0) ([#41](https://github.com/woodleighschool/ADOverseas/issues/41)) ([5e4b046](https://github.com/woodleighschool/ADOverseas/commit/5e4b04667a2f47701fe82a32483c6473ab31abca))
* **deps:** update dependency globals (17.6.0 → 17.7.0) ([#42](https://github.com/woodleighschool/ADOverseas/issues/42)) ([501bb04](https://github.com/woodleighschool/ADOverseas/commit/501bb040a4e136a5f044478074c23d1992c836db))
* **deps:** update dependency prettier (3.8.3 → 3.9.5) ([#43](https://github.com/woodleighschool/ADOverseas/issues/43)) ([1b8c07f](https://github.com/woodleighschool/ADOverseas/commit/1b8c07f8d4d5eb570b9e6eaf839635e7ce09dc5e))
* **deps:** update dependency react-hook-form (7.76.1 → 7.81.0) ([#44](https://github.com/woodleighschool/ADOverseas/issues/44)) ([0770bb8](https://github.com/woodleighschool/ADOverseas/commit/0770bb87c8c3abf4b811eff68fb980d4a60aa5cd))
* **deps:** update dependency react-router-dom (7.15.1 → 7.18.1) ([#45](https://github.com/woodleighschool/ADOverseas/issues/45)) ([84193fe](https://github.com/woodleighschool/ADOverseas/commit/84193fef1fec518a88973f1b7999097d04ce0cb1))
* **deps:** update dependency typescript-eslint (8.60.0 → 8.64.0) ([#46](https://github.com/woodleighschool/ADOverseas/issues/46)) ([7e5e90a](https://github.com/woodleighschool/ADOverseas/commit/7e5e90a6c6802f52e07a9d10f01914bbca3b961f))
* **deps:** update dependency vite (8.0.16 → 8.1.5) ([#61](https://github.com/woodleighschool/ADOverseas/issues/61)) ([72d2cf2](https://github.com/woodleighschool/ADOverseas/commit/72d2cf2bff6016d6125efc3ecffd9de2f2d81a98))
* **deps:** update material-ui monorepo ([#47](https://github.com/woodleighschool/ADOverseas/issues/47)) ([f2be1a2](https://github.com/woodleighschool/ADOverseas/commit/f2be1a2cfdd017665e9f622f9acafae0be784667))
* **deps:** update module github.com/azure/azure-sdk-for-go/sdk/azidentity (v1.13.1 → v1.14.0) ([#48](https://github.com/woodleighschool/ADOverseas/issues/48)) ([aeee94e](https://github.com/woodleighschool/ADOverseas/commit/aeee94ed9f642189cccafa3648d85bd387f9c781))
* **deps:** update module github.com/coreos/go-oidc/v3 (v3.18.0 → v3.20.0) ([#49](https://github.com/woodleighschool/ADOverseas/issues/49)) ([8f31378](https://github.com/woodleighschool/ADOverseas/commit/8f31378cf311894a463a6e84b40b3713d2ffdf64))
* **deps:** update module github.com/jackc/pgx/v5 (v5.9.2 → v5.10.0) ([#50](https://github.com/woodleighschool/ADOverseas/issues/50)) ([0c44c8d](https://github.com/woodleighschool/ADOverseas/commit/0c44c8dabe7d214417b3a3f9cfcc6c599f542b68))
* **deps:** update module github.com/microsoftgraph/msgraph-sdk-go (v1.99.0 → v1.100.0) ([#51](https://github.com/woodleighschool/ADOverseas/issues/51)) ([c3d7d62](https://github.com/woodleighschool/ADOverseas/commit/c3d7d62690f79cc7e852360522b9cda84d30bd7a))
* **deps:** update node.js (v25.2.1 → v25.9.0) ([#52](https://github.com/woodleighschool/ADOverseas/issues/52)) ([cf0423f](https://github.com/woodleighschool/ADOverseas/commit/cf0423fb37dcce4d6e27ebeb0a10880e0d4f213f))


### Bug Fixes

* **ci:** adopt release please ([14c1c3a](https://github.com/woodleighschool/ADOverseas/commit/14c1c3ae5ba7a1d7fa933f29689cf185a3ef9c43))
* **deps:** update dependency @vitejs/plugin-react (6.0.2 → 6.0.3) ([#32](https://github.com/woodleighschool/ADOverseas/issues/32)) ([1502640](https://github.com/woodleighschool/ADOverseas/commit/15026409d622d4a8d2f2ad7913497de64bf24c6e))
* **deps:** update dependency eslint-plugin-react-refresh (0.5.2 → 0.5.3) ([#33](https://github.com/woodleighschool/ADOverseas/issues/33)) ([8da3090](https://github.com/woodleighschool/ADOverseas/commit/8da30904ac7c8ec057bc2c24d8be7336d4a85bb8))
* **deps:** update dependency react-virtuoso (4.18.7 → 4.18.10) ([#34](https://github.com/woodleighschool/ADOverseas/issues/34)) ([41e935f](https://github.com/woodleighschool/ADOverseas/commit/41e935ff8273f571f30be53486cb96cebd81d45a))
* **deps:** update dependency vite (8.0.14 → 8.0.16) [security] ([#55](https://github.com/woodleighschool/ADOverseas/issues/55)) ([8229565](https://github.com/woodleighschool/ADOverseas/commit/8229565016142ba760498ab756554ba6afe8a389))
* **deps:** update module github.com/go-chi/chi/v5 (v5.3.0 → v5.3.1) ([#35](https://github.com/woodleighschool/ADOverseas/issues/35)) ([9ec7d58](https://github.com/woodleighschool/ADOverseas/commit/9ec7d5807c7f4ab210645b4d120424dbe191689b))
* **deps:** update react monorepo ([#36](https://github.com/woodleighschool/ADOverseas/issues/36)) ([28b5393](https://github.com/woodleighschool/ADOverseas/commit/28b5393fab00fd7f22b5625ba0c47cf6cec51be6))


### Code Refactoring

* flatten application layout ([cd6b3f5](https://github.com/woodleighschool/ADOverseas/commit/cd6b3f5050226278ace4dff49ff9a9e5d5211316))
