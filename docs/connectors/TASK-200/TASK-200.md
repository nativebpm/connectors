---
task: TASK-200
status: Completed
summary: Update Gotenberg Go client for version 8.34.0, adding PDF ownerPassword and permission controls, and Factur-X/ZUGFeRD support.
---

# TASK-200: Update Gotenberg Client to 8.34.0

## Description

Gotenberg 8.34.0 was released with new features and security hardening:
1. **Security Fixes**: Blocking untrusted referer links in LibreOffice (automatically managed by container).
2. **OwnerPassword**: Independent ownerPassword for owner-only encryption and permission controls.
3. **Permission Controls**: Boolean flags (`allowPrinting`, `allowCopying`, `allowModifying`, `allowAnnotating`, `allowFillingForms`, `allowAssembling`).
4. **Factur-X / ZUGFeRD support**: Dedicated route `/forms/pdfengines/factur-x` and parameters (`facturxXml`, `facturxConformanceLevel`, `facturxDocumentType`, `facturxVersion`).
5. **PDF Embedding Route**: Dedicated route `/forms/pdfengines/embed` and `embeds` parameter for embedding files.

We need to update our Go client library (`gotenberg` package) to support these features and add tests checking compatibility with version `8.34.0`.
