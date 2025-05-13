# Security Policy

## Supported Versions

Use this section to tell people about which versions of your project are currently being supported with security updates.

| Version | Supported          |
| ------- | ------------------ |
| 0.1.0   | :white_check_mark: |

## Reporting a Vulnerability

The KubeRocketCI team takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose your findings.

To report a security vulnerability, please follow these steps:

1. **DO NOT** disclose the vulnerability publicly on GitHub Issues or other public forums.
2. Email us at <SupportEPMD-EDP@epam.com> with a detailed description of the vulnerability.
3. Include steps to reproduce, impact, and any potential mitigations if known.
4. Allow time for the team to investigate and address the vulnerability before any public disclosure.

## What to expect

- Acknowledgment of your report within 48 hours
- An initial assessment of the report within 7 days
- Regular updates about the progress of addressing the vulnerability
- Credit for discovering the vulnerability (unless you prefer to remain anonymous)

## Disclosure Policy

We follow a coordinated disclosure process:

1. Once a vulnerability is confirmed, we develop and test a fix
2. We prepare a security advisory to accompany the fix
3. We release the fix and publish the security advisory simultaneously
4. After the fix has been available for 10 days, details may be discussed publicly

## Security Best Practices

When deploying GitFusion:

1. Use the principle of least privilege for the service account
2. Keep your Kubernetes environment updated with security patches
3. Protect API keys and tokens used for Git provider authentication
4. Regularly review and audit access to GitFusion and its resources
5. Follow security best practices for handling Git credentials

Thank you for helping us keep our project and our users secure!
