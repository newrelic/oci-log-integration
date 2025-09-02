package common

// OCILoggingEvent represents a collection of OCI log entries as JSON strings.
// Each string in the slice contains a JSON-encoded log entry from OCI Logging service.
type OCILoggingEvent []map[string]interface{}