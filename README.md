# zapotel

[Zap](https://github.com/uber-go/zap) helpers for writing [opentelemetry formatted logs](https://opentelemetry.io/docs/reference/specification/logs/data-model/).

## Installation

`go get github.com/bakins/zapotel`

## Usage

```go
package main

import (
	"github.com/bakins/zapotel"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
    "go.opentelemetry.io/otel/sdk/resource"
)

func main() {
    // Will write json to stdout
    logger := zapotel.NewLogger(zapcore.InfoLevel).Named("testing")

    // add opentelemetry resource metadata
    logger = logger.With(zapotel.Resource(resource.Environment()))

    logger.Info("this is a message", zap.String("field", "value"))
}
```

Will print all on one line (formatted for easier viewing):

```json
{
  "severity_text": "INFO",
  "timestamp": 1655157075426794000,
  "scope_name": "testing",
  "body": "this is a message",
  "severity": 9,
  "resource": {
    "hostname": "localhost"
  },
  "attributes": {
    "field": "value"
  }
}
```

## See also

* https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/f531a708efce0e43cc4e999fd66f5ee7411e9c0e/pkg/stanza/entry/entry.go#L25


## LICENSE

See [LICENSE](./LICENSE)
