---
name: ato-compliant-infrastructure
description: Use when generating Terraform or other infrastructure-as-code for federal workloads that must stay NIST 800-53 aligned from the first iteration, especially when demo timelines, LocalStack limits, or stakeholder pressure tempt shortcuts.
version: '0.5.0'
---

# ATO-Compliant Infrastructure

## Overview
Keep ATO and FedRAMP expectations front-of-mind even in LocalStack demos or rapid spikes. Apply the same NIST 800-53 security patterns, tagging, and evidence capture you would ship to production so prototypes stay promotable and audit-friendly.

Keywords: ATO, NIST 800-53, FedRAMP, Terraform, LocalStack, compliance, evidence, SSP, cATO readiness, security automation.

## When to Use
- Stakeholders demand “quick Terraform” for a federal workload, demo, or proof-of-concept.
- LocalStack or other constrained environments tempt you to skip encryption, tagging, or documentation.
- You must show that infrastructure changes map to specific controls or SSP sections.
- Security reviewers, ISSOs, or AOs expect controls evidence alongside code delivery.
- You are unsure how to reconcile LocalStack limitations with production security patterns.

Do not use for non-regulated hobby projects or when another agency-specific guide supersedes the NIST baseline.

## Core Pattern
1. **Anchor on controls:** List impacted controls (e.g., SC-28, AU-9) before writing any Terraform.
2. **Mirror production guardrails:** Configure providers, encryption, logging, and networking exactly as production would, adjusting only where LocalStack lacks support.
3. **Tag and comment for evidence:** Capture control IDs, SSP references, and compliance level in tags and inline comments.
4. **Generate documentation outputs:** Produce artifacts (tables, CSVs, markdown) mapping resources to controls and evidence locations.
5. **Validate demo constraints:** Confirm features stay within LocalStack Community capabilities; note deviations explicitly.
6. **Record assurances:** Summarize how choices satisfy ATO reviewers and flag follow-ups when LocalStack can only simulate behavior.

## Quick Reference
| Stage | Action | Key Controls | Notes |
| --- | --- | --- | --- |
| Provider setup | Use AWS provider overrides with `s3_use_path_style`, skip validations, set compliance tags | CM-2, CM-6 | Reference LocalStack endpoints, include default tags for ComplianceFramework |
| Data protection | Enable SSE, enforce TLS, lock bucket policies | SC-13, SC-28, AC-6 | AES256 works in LocalStack free tier; document KMS gaps |
| Audit & logging | Create CloudTrail/S3 logs stand-ins, document limitations | AU-2, AU-9, CP-9 | If service absent, state simulated behavior and follow-up |
| Access control | Use least-privilege IAM policies or Kubernetes RBAC | AC-2, AC-6, IA-2 | Document any wildcard usage and remediation plan |
| Evidence outputs | Emit tables/csv linking controls to Terraform resources | PL-2, CA-7 | Store alongside code in `docs/` or `outputs/` |

## Implementation Notes
### Provider Baseline
- Require Terraform >= 1.5 and AWS/Kubernetes providers pinned (hashicorp/aws ~> 5.0, hashicorp/kubernetes ~> 2.23).
- Configure LocalStack endpoints at `http://localhost:4566`, set `s3_use_path_style = true`, and skip credential/account checks.
- Apply default tags for `ComplianceFramework`, `AtoCriticality`, `DataClassification`, and `ManagedBy`.

### Resource Patterns
- Encrypt S3 buckets via `aws_s3_bucket_server_side_encryption_configuration` (AES256) and attach bucket policies that deny unencrypted traffic.
- Enable versioning and lifecycle backups with explicit `filter` blocks to satisfy CP-9 and AU-9.
- Define IAM policies with least privilege; avoid `"Action": "*"`. Tag policy documents with control references.
- For Kubernetes, use `NetworkPolicy` objects rather than relying on open traffic; note LocalStack/KIND networking constraints.

### Evidence Automation
- Use Terraform outputs to list resource ARNs, control mappings, and evidence file paths.
- Generate markdown or CSV control matrices summarizing implementations (control, resource, evidence location, status).
- Annotate TODOs where LocalStack cannot enforce a control (e.g., KMS CMKs); include remediation notes for real AWS.

## Example
```hcl
# File: s3.tf
# Controls: SC-28, SC-28-1, AU-9, CP-9 | SSP Section: 10.3.1 Data Encryption
resource "aws_s3_bucket" "app_storage" {
  bucket = var.bucket_name

  tags = {
    Name                = var.bucket_name
    ComplianceControls  = "SC-28_SC-28-1_AU-2_AU-9_CP-9_AC-6"
    ComplianceFramework = "NIST-800-53"
    ComplianceLevel     = "moderate"
    AtoCriticality      = "high"
    DataClassification  = "sensitive"
    ManagedBy           = "OpenTofu"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "app_storage" {
  bucket = aws_s3_bucket.app_storage.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256" # LocalStack Community supports AES256
    }
  }
}

resource "aws_s3_bucket_versioning" "app_storage" {
  bucket = aws_s3_bucket.app_storage.id

  versioning_configuration {
    status = "Enabled"
  }
}
```

## Rationalization Table
| Rationalization | Why It Fails | Countermeasure |
| --- | --- | --- |
| "It's just a LocalStack demo; encryption can wait." | ATO reviewers expect parity with production patterns; skipping controls creates rework and breaks cATO posture. | Default to production-grade encryption and document any simulated pieces. |
| "Tags and evidence outputs slow us down." | Missing metadata blocks SSP generation and audit trails, forcing manual reconciliation later. | Bake tags/comments into templates and reuse them; automation saves time overall. |
| "LocalStack doesn't support feature X, so drop the control." | Removing controls hides the gap instead of tracking it for remediation in real AWS. | Implement the intent, note the limitation, and flag follow-up tasks for full environments. |

## Red Flags
- Requests to remove encryption, logging, or tagging "just for speed."
- Wildcard IAM policies without documented justification.
- Missing notes about LocalStack limitations or future remediation paths.
- No Terraform outputs or docs explaining control coverage.
- Stakeholders insisting evidence can be "handled later."

## Common Mistakes
- Treating demos as exempt from compliance, leading to throwaway work.
- Forgetting `s3_use_path_style = true`, causing broken LocalStack S3 integration.
- Using complex regex validations or LocalStack Pro-only services that fail locally.
- Leaving TODOs without linking them to control IDs or remediation owners.
- Mixing control identifiers (e.g., `SC-28(1)` vs `SC-28-1`) making evidence parsing brittle.

## Verification
- Re-run pressure scenarios: ensure you still insist on encryption/tagging and produce evidence outputs despite time pressure.
- Confirm every control citation maps to resources or documented limitations.
- Hand off markdown/CSV artifacts with clear control/resource/evidence mapping.

