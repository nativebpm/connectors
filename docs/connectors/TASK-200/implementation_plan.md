# Update Gotenberg Client to v8.34.0

This plan describes the changes required to update our Gotenberg client library (`gotenberg` package) to support Gotenberg v8.34.0 features, specifically:
- PDF ownerPassword and permission controls
- PDF embedding route and embeds files
- Factur-X / ZUGFeRD e-invoicing route and dedicated form fields

## User Review Required

> [!NOTE]
> All new methods are added using fluent builders returning the respective builder types, maintaining API compatibility with the existing client patterns.

## Open Questions

None. The API is straightforward and directly maps to Gotenberg's form parameters.

## Proposed Changes

---

### Gotenberg Client library (`gotenberg` package)

#### [MODIFY] [gotenberg.go](file:///Users/user/github.com/nativebpm/connectors/gotenberg/gotenberg.go)
- Add base parameters and methods to `Request` type for PDF security and embedding:
  - `UserPassword(password string) *Request`
  - `OwnerPassword(password string) *Request`
  - `AllowPrinting(allow bool) *Request`
  - `AllowCopying(allow bool) *Request`
  - `AllowModifying(allow bool) *Request`
  - `AllowAnnotating(allow bool) *Request`
  - `AllowFillingForms(allow bool) *Request`
  - `AllowAssembling(allow bool) *Request`
  - `Embeds(filename string, content io.Reader) *Request`

#### [MODIFY] [chromium.go](file:///Users/user/github.com/nativebpm/connectors/gotenberg/chromium.go)
- Wrap the new `Request` methods so they can be chained on `Chromium` builder:
  - `UserPassword`, `OwnerPassword`, `AllowPrinting`, `AllowCopying`, `AllowModifying`, `AllowAnnotating`, `AllowFillingForms`, `AllowAssembling`, `Embeds`.

#### [MODIFY] [libreoffice.go](file:///Users/user/github.com/nativebpm/connectors/gotenberg/libreoffice.go)
- Wrap the new `Request` methods so they can be chained on `LibreOffice` builder. Note: LibreOffice already has a `Password(password string)` method which sets the source document password. The new `UserPassword` and `OwnerPassword` methods will set the PDF security passwords.

#### [MODIFY] [pdfengines.go](file:///Users/user/github.com/nativebpm/connectors/gotenberg/pdfengines.go)
- Wrap the new `Request` methods so they can be chained on `PDFEngines` builder.
- Add support for the new `/forms/pdfengines/embed` route:
  - `Embed(ctx context.Context) *PDFEngines`
- Add support for the new `/forms/pdfengines/factur-x` route and its dedicated fields:
  - `FacturX(ctx context.Context) *PDFEngines`
  - `FacturXXml(filename string, content io.Reader) *PDFEngines`
  - `FacturXConformanceLevel(level string) *PDFEngines`
  - `FacturXDocumentType(docType string) *PDFEngines`
  - `FacturXVersion(version string) *PDFEngines`

#### [MODIFY] [test_all_versions.sh](file:///Users/user/github.com/nativebpm/connectors/gotenberg/test_all_versions.sh)
- Add `"8.34.0"` to the `VERSIONS` array to run full compatibility test suite against the new version.

## Verification Plan

### Automated Tests
- Run `test_all_versions.sh` script to verify existing and new functionality on Docker containers across all supported versions, including the new `8.34.0` release:
  ```bash
  cd gotenberg && ./test_all_versions.sh
  ```
- Write a unit/integration test for the new methods in `gotenberg_test.go` or within the examples/tests flow.
