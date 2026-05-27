# ACTIVE_TASK

## ID: Task 2
## Title: APNS private key file option
## Issue: #10

### Description

Add support for specifying the APNS (Apple Push Notification Service) private key from a file path instead of requiring it as a base64-encoded environment variable. This makes configuration easier in production deployments where files are mounted via secrets.

### Implementation target

- `internal/config/` configuration loading
- `internal/pushnotify/` APNS handler
- Likely: config validation and default behavior

### Completion criteria

- [ ] go test ./... passes
- [ ] APNS can load private key from file path (verify in config test or integration test)
- [ ] APNS still works with base64-encoded inline keys for backward compatibility
- [ ] docs/CURRENT_STATUS.md updated with Task 2 completion
- [ ] docs/backend-roadmap.md marked as complete
- [ ] All changes committed and pushed

### Next task

Task 3: Helm CHANGEME guard

### Notes

This is a configuration enhancement to support file-based secret management, which is common in K8s deployments.
