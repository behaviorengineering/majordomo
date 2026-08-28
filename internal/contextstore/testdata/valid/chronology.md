# Chronology

Newest first.

### 2026-08-28 - Alice - PR #412

- **Did:** extract auth into middleware
- **Because:** token checks were duplicated in three handlers
- **In order to:** make session expiry consistent
- **Evidence:** PR #412, merge `abc123def`

### 2026-08-01 - Bob - PR #400

- **Did:** split billing from checkout
- **Because:** the monolith mixed payment and cart state
- **In order to:** deploy billing independently
- **Evidence:** PR #400, merge `def456abc`
