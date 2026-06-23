# Walkthrough: Gotenberg Client Update to v8.34.0

This walkthrough summarizes the changes made to update the Gotenberg client library (`gotenberg` package) to support Gotenberg v8.34.0.

## Changes Made

### 1. Request Base Configuration (`gotenberg.go`)
- Added methods to configure PDF security and encryption (passwords and permissions):
  - `UserPassword(password string)`
  - `OwnerPassword(password string)`
  - `AllowPrinting(allow bool)`
  - `AllowCopying(allow bool)`
  - `AllowModifying(allow bool)`
  - `AllowAnnotating(allow bool)`
  - `AllowFillingForms(allow bool)`
  - `AllowAssembling(allow bool)`
- Added `Embeds(filename string, content io.Reader)` to upload files with the `embeds` form name for document embedding.

### 2. Chromium and LibreOffice Builders (`chromium.go`, `libreoffice.go`)
- Wrapped the new `Request` security and embeds methods to allow fluent method chaining.

### 3. PDF Engines Builder (`pdfengines.go`)
- Wrapped all new security and embeds methods.
- Added `/forms/pdfengines/embed` route:
  - `Embed(ctx context.Context)`
- Added `/forms/pdfengines/factur-x` route for Factur-X / ZUGFeRD e-invoicing:
  - `FacturX(ctx context.Context)`
  - `FacturXXml(filename string, content io.Reader)`
  - `FacturXConformanceLevel(level string)`
  - `FacturXDocumentType(docType string)`
  - `FacturXVersion(version string)`

### 4. Tests and Benchmarks (`gotenberg_test.go`)
- Added `TestChromiumSecurityAndEmbeds`, `TestLibreOfficeSecurityAndEmbeds`, `TestPDFEnginesEmbed`, and `TestPDFEnginesFacturX` verifying correct HTTP serialization and endpoint mapping.
- Added benchmarks for builder configuration chains showing high performance and low allocations.

### 5. Version Compatibility (`test_all_versions.sh`)
- Appended version `"8.34.0"` to the list of tested versions.

---

## Verification Results

### Unit & Benchmark Tests
All unit tests passed successfully. The benchmark output shows excellent performance:
- `BenchmarkChromiumBuilder-10`: **530.0 ns/op**, **2096 B/op**, **11 allocs/op**
- `BenchmarkPDFEnginesBuilder-10`: **536.1 ns/op**, **2088 B/op**, **11 allocs/op**

### Compatibility Test Suite
Running the full version compatibility test suite across all supported docker images.
