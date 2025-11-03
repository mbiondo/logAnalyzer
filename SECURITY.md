# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of LogAnalyzer seriously. If you discover a security vulnerability, please follow these steps:

### 1. **Do Not** Open a Public Issue

Please do not report security vulnerabilities through public GitHub issues.

### 2. Report Privately

Send a description of the vulnerability to: **biondo.maximiliano@gmail.com**

Or report it privately through [GitHub Security Advisories](https://github.com/mbiondo/logAnalyzer/security/advisories/new)

Include:
- Type of vulnerability
- Full description with steps to reproduce
- Potential impact
- Suggested fix (if any)

### 3. What to Expect

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 7 days
- **Status Updates**: Every 7 days
- **Fix Timeline**: Depends on severity
  - Critical: 1-7 days
  - High: 7-30 days
  - Medium: 30-90 days
  - Low: 90+ days

### 4. Disclosure Policy

- We will work with you to understand and resolve the issue
- We will credit you in the security advisory (unless you prefer to remain anonymous)
- We will publish a security advisory after the fix is released
- Please allow us reasonable time to address the issue before public disclosure

## Security Best Practices

When deploying LogAnalyzer:

### Container Security

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
```

### Configuration Security

1. **Never commit secrets** to configuration files
2. **Use environment variables** or secret managers for sensitive data
3. **Restrict file permissions**: `chmod 600 config.yaml`
4. **Use HTTPS** for all webhooks and external services
5. **Rotate credentials** regularly

### Network Security

1. **Limit exposed ports**: Only expose necessary ports
2. **Use network policies**: In Kubernetes, restrict pod-to-pod communication
3. **TLS/SSL**: Use encrypted connections for all external integrations

### Access Control

1. **RBAC**: Use least privilege principle in Kubernetes
2. **Service accounts**: Create dedicated service accounts
3. **Audit logs**: Enable logging for security events

## Known Security Considerations

### Docker Socket Access

When running as a DaemonSet with Docker socket access:

```yaml
# Use with caution - this grants container-level access
volumeMounts:
- name: docker-socket
  mountPath: /var/run/docker.sock
  readOnly: true
```

**Risk**: Container escape vulnerability if LogAnalyzer is compromised

**Mitigation**:
- Use read-only mount
- Run with minimal privileges
- Use alternatives like containerd CRI when possible
- Implement network segmentation

### Log Content

Logs may contain sensitive information:

- **PII** (Personally Identifiable Information)
- **Credentials** accidentally logged
- **API keys** or tokens

**Mitigation**:
- Implement log masking/redaction
- Use regex filters to remove sensitive patterns
- Encrypt logs in transit and at rest
- Apply data retention policies

### Webhook Security

For Slack, HTTP, and other webhook integrations:

**Risks**:
- Man-in-the-middle attacks
- Webhook URL exposure
- Replay attacks

**Mitigation**:
- Always use HTTPS
- Store webhook URLs as secrets
- Implement request signing when available
- Rate limit webhook calls
- Validate SSL certificates

## Security Features

LogAnalyzer includes several security features:

- ✅ **No elevated privileges required** (except DaemonSet mode)
- ✅ **Read-only filesystem** support
- ✅ **Secret-free configuration** via environment variables
- ✅ **Minimal Docker image** (FROM scratch)
- ✅ **No runtime dependencies**
- ✅ **Static binary compilation**

## Vulnerability Scanning

We use automated security scanning:

- **Trivy**: Container image scanning
- **Dependabot**: Dependency updates
- **CodeQL**: Static code analysis
- **gosec**: Go security checker

### gosec Security Scanning

LogAnalyzer uses [gosec](https://github.com/securego/gosec) for automated security scanning of Go code. gosec is a security linter that identifies potential security issues in Go code.

#### How gosec Works

- Scans Go source code for security problems
- Uses static analysis of Go AST and SSA representations
- Identifies 60+ security issues including:
  - Hard-coded credentials
  - SQL injection vulnerabilities
  - Path traversal attacks
  - Weak cryptographic practices
  - Insecure TLS configurations

#### #nosec Comments

When gosec identifies a potential security issue that we've validated as safe in our specific context, we use `#nosec` comments to suppress the warning:

```go
// This is acceptable for development environments only
InsecureSkipVerify: true, // #nosec G402
```

**Important**: `#nosec` comments are not blind suppressions. Each one represents:
- Code review and validation that the pattern is safe in our context
- Additional security measures implemented elsewhere
- Clear documentation of why the code is secure

#### Current Security Status

- ✅ **0 security issues** found by gosec
- ✅ **8 validated suppressions** (`#nosec` comments)
- ✅ **All suppressions documented** with security justifications

#### CI/CD Integration

gosec runs automatically on:
- Every push to `main` and `develop` branches
- All pull requests
- Release builds

The CI pipeline fails if new security issues are introduced (excluding validated `#nosec` suppressions).

## Security Updates

Security updates are released as:
- **Patch versions** for minor fixes
- **Security advisories** on GitHub
- **Docker image tags** with security patches

Subscribe to:
- [GitHub Security Advisories](https://github.com/mbiondo/logAnalyzer/security/advisories)
- [Release notifications](https://github.com/mbiondo/logAnalyzer/releases)
- RSS feed: `https://github.com/mbiondo/logAnalyzer/releases.atom`

## Responsible Disclosure Examples

We appreciate responsible disclosure from:

- Security researchers
- Users who discover vulnerabilities
- Automated security tools

Thank you for helping keep LogAnalyzer and our users safe! 🔒
