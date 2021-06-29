# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [v2.1.0] - 2021-06-29

### Added

- *: Implement GSP-87 Feature Gates (#26)
- storage: Implement GSP-93 Add ObjectMode Pair (#31)
- storage: Implement GSP-97 Add Restrictions In Storage Metadata (#31)

### Changed

- *: Implement GSP-109: Redesign Features (#31)
- *: Implement GSP-117 Rename Service to System as the Opposite to Global (#31)

### Fixed

- storage: Fix listMultipart cannot get complete uploaded parts (#28)

## [v2.0.0] - 2021-05-24

### Added

- storage: Implement appender support (#15)
- storage: Implement CommitAppend (#18)
- *: Implement GSP-47 & GSP-51 (#22)
- storage: Implement Multipart support (#21)
- storage: Implement GSP-61 Add object mode check for operations (#23)

### Changed

- storage: Idempotent storager delete operation (#20)
- *: Implement GSP-73 Organization rename (#24)

## [v1.1.0] - 2021-04-24

### Added

- *: Implement default pair support for service (#5)
- storage: Implement Create API (#11)
- *: Add UnimplementedStub (#12)
- tests: Introduce STORAGE_OSS_INTEGRATION_TEST (#13)
- storage: Implement SSE support (#14)
- storage: Implement GSP-40 (#16)

### Changed

- ci: Only run Integration Test while push to master

### Upgrade

- Bump github.com/aliyun/aliyun-oss-go-sdk (#9)

## v1.0.0 - 2021-02-07

### Added

- Implement oss services.

[v2.1.0]: https://github.com/beyondstorage/go-service-oss/compare/v2.0.0...v2.1.0
[v2.0.0]: https://github.com/beyondstorage/go-service-oss/compare/v1.1.0...v2.0.0
[v1.1.0]: https://github.com/beyondstorage/go-service-oss/compare/v1.0.0...v1.1.0
