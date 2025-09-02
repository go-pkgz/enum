Example: status enum

This example shows how to generate enums with optional integrations.

Current files were generated previously; to regenerate with the new flags, run from this folder:

```
# JSON only (default TextMarshaler/Unmarshaler, no extra deps)
go run ../../main.go -type status -lower

# Add SQL support
go run ../../main.go -type status -lower -sql

# Add MongoDB BSON support
go run ../../main.go -type status -lower -bson

# Add YAML support
go run ../../main.go -type status -lower -yaml

# Combine as needed, e.g. BSON + SQL
go run ../../main.go -type status -lower -bson -sql
```

Notes
- `-bson` uses mongo-go-driver BSON interfaces; values are stored as strings.
- `-sql` implements `driver.Valuer` and `sql.Scanner`.
- `-yaml` implements `yaml.Marshaler`/`yaml.Unmarshaler` (gopkg.in/yaml.v3).

