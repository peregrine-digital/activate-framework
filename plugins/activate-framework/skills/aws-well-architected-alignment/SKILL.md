---
name: aws-well-architected-alignment
description: Use when planning, reviewing, or remediating AWS workloads that must follow the Well-Architected Framework, especially when deadline, cost, or lift-and-shift pressure tempts teams to skip pillar-level best practices.
version: '0.5.0'
---

# AWS Well-Architected Alignment

## Overview
Keep every AWS workload tied to the six Well-Architected pillars—Operational Excellence, Security, Reliability, Performance Efficiency, Cost Optimization, and Sustainability—from the first architecture sketch through ongoing operations. Treat the Well-Architected review as a continuous governance loop: surface risks, decide on remediations, and make the trade-offs visible.

Keywords: Well-Architected Review, Operational Excellence, Security, Reliability, Performance Efficiency, Cost Optimization, Sustainability, AWS WA Tool, lenses, action items, resiliency, cost governance.

## When to Use
- Product or platform teams are designing, migrating, or scaling an AWS workload.
- Leadership pushes for rapid launch and suggests postponing Well-Architected reviews.
- Finance or product pressure encourages removing resilience, security, or sustainability guardrails for short-term savings.
- Teams are copying on-premises runbooks or infrastructure to AWS without modernization.
- You need a repeatable way to capture risks, build remediation backlogs, and justify trade-offs to auditors or executives.

Skip this skill only when the workload is non-critical playground infrastructure with no customers, compliance, or cost exposure.

## Core Pattern
1. **Clarify business context:** Define workload purpose, criticality, RTO/RPO expectations, budget, carbon goals, and stakeholder tolerances.
2. **Map to pillars:** For each pillar, enumerate the AWS design principles and identify how the workload satisfies or violates them.
3. **Use structured questions:** Walk through the AWS Well-Architected question set (or relevant lens) and capture risk notes, metrics, and owners.
4. **Translate to action items:** Convert each high/medium risk into backlog tasks with severity, success criteria, and target date.
5. **Embed feedback loop:** Track remediation progress, schedule recurring reviews (at least quarterly or after major changes), and share outcomes.

## Quick Reference
| Stage | Actions | Pillars Emphasized | Artifacts |
| --- | --- | --- | --- |
| Discovery | Capture workload context, business KPIs, compliance scope | Operational Excellence, Cost Optimization | Context brief, architectural diagrams |
| Pillar deep dive | Apply design principles and question sets; document risks | All six pillars | Risk log, pillar scorecards |
| Prioritization | Rank remediation tasks by blast radius, cost, probability | Security, Reliability, Cost Optimization | Prioritized backlog, owners |
| Implementation | Execute improvements, measure impact, update runbooks | Operational Excellence, Performance Efficiency, Sustainability | Change records, metrics dashboards |
| Governance | Re-review after changes, report posture to stakeholders | Security, Sustainability, Operational Excellence | Quarterly review deck, audit evidence |

## Implementation Notes
### Operational Excellence
- Perform operations as code: enforce Infrastructure as Code with review gates and version control.
- Make routine adjustments small: use runbooks, SOPs, and automation to iterate frequently.
- Learn from events: run post-incident reviews, capture lessons learned, and update playbooks.

### Security
- Enable traceability: aggregate logs (CloudTrail, CloudWatch, AWS Config) and guardrails (AWS Control Tower, SCPs).
- Apply least privilege everywhere: use IAM roles with scoped policies, rotate credentials automatically.
- Automate security best practices: leverage AWS Security Hub, GuardDuty, and detective controls; encrypt data in transit and at rest.

### Reliability
- Automatically recover from failure: use multi-AZ, multi-Region where critical, health checks, and auto scaling.
- Test recovery procedures: run game days, chaos experiments, and DR simulations.
- Manage change and quotas: versioned deployments (CI/CD), AWS Service Quotas monitoring, dependency mapping.

### Performance Efficiency
- Use data to drive architecture: observe key metrics (latency, throughput, resource utilization).
- Adopt managed and serverless services when possible to minimize undifferentiated heavy lifting.
- Experiment frequently: run load tests, compare instance families, use AWS Compute Optimizer insights.

### Cost Optimization
- Implement financial guardrails: budgets, cost anomaly detection, cost allocation tagging.
- Match supply to demand: right-size instances, adopt Auto Scaling, turn off idle resources.
- Choose pricing models deliberately: Savings Plans, Reserved Instances, Spot where appropriate.

### Sustainability
- Maximize utilization: choose efficient instance types, consolidate workloads, prefer managed services.
- Set sustainability KPIs: track energy/carbon impact metrics and include sustainability goals in governance.
- Anticipate new tech: evaluate Graviton, data tiering, and architectural patterns that reduce resource usage.

### Tooling & Evidence
- Use the AWS Well-Architected Tool to capture answers, assign owners, and track improvements.
- Apply domain-specific lenses (Serverless, SaaS, ML, Sustainability) when workloads require deeper guidance.
- Maintain a single risk register that links question IDs to remediation tasks, metrics, and status.

## Example
```markdown
| Pillar | Question ID | Risk | Action Item | Owner | Target Date |
| --- | --- | --- | --- | --- | --- |
| Reliability | REL 10 | Multi-AZ disabled for primary database | Enable Aurora global database, run failover drill | DB SRE Lead | 2025-01-15 |
| Security | SEC 5 | No centralized IAM policy review | Deploy IAM Access Analyzer, establish quarterly review | Platform Security | 2024-12-01 |
| Cost Optimization | COST 3 | Idle m5.4xlarge fleet outside business hours | Implement instance scheduler and budgets alarm | FinOps | 2024-11-20 |
| Sustainability | SUS 2 | Workload on older x86 instances | Benchmark Graviton3 adoption, update KPI dashboard | Sustainability WG | 2025-02-28 |
```

## Rationalization Table
| Rationalization | Why It Fails | Countermeasure |
| --- | --- | --- |
| "We have a launch deadline—skip the Well-Architected review this time." | Unaddressed risks resurface as outages, security gaps, and post-launch rework that cost more than an on-time review. | Schedule a lightweight pillar checkpoint (2 hours) before launch, document deferred risks with owners and dates. |
| "Finance wants to drop multi-AZ and backups to cut costs." | Sacrificing redundancy breaks Reliability and Security pillars and jeopardizes RTO/RPO commitments, exposing larger financial and compliance risk. | Present total cost of failure, highlight cost-optimization alternatives (rightsizing, Savings Plans) that preserve resilience. |
| "We're just lifting-and-shifting; automation and sustainability can wait." | Manual operations contradict Operational Excellence and Sustainability, leading to errors, toil, and higher carbon and staffing costs. | Require IaC and runbook updates as part of migration definition-of-done, set sustainability KPIs alongside migration milestones. |
| "The AWS Well-Architected Tool is too heavy—let's track actions in a wiki later." | Losing the structured tool breaks traceability, governance dashboards, and executive reporting, so risks go stale or disappear. | Enter answers directly in the WA Tool, export action reports for the wiki, and assign owners during the session to keep data authoritative. |

## Red Flags
- No documented workload context or criticality before the review starts.
- Pillars discussed only informally with no written risks or remediation backlog.
- Decisions made solely on cost with no RTO/RPO, security, or sustainability evaluation.
- "To-do later" items lack owners or dates, indicating no governance enforcement.
- Teams skipping AWS Well-Architected Tool because it feels “too heavy.”

## Common Mistakes
- Treating the framework as a one-time audit instead of an ongoing improvement cycle.
- Copying answers from past reviews without validating current architecture.
- Ignoring Sustainability because it is “nice to have” rather than tying it to business KPIs.
- Mixing control IDs or question references, making evidence hard to trace.
- Overloading the review with every action item at once instead of prioritizing biggest risks.

## Verification
- Re-run the original pressure scenarios: ensure the team schedules a review, keeps resilience/security guardrails, and modernizes operations despite pressures.
- Confirm every high-risk item carries an owner, success metric, and target remediation window.
- Demonstrate that results feed into regular governance (e.g., quarterly reports, security reviews, sustainability dashboards).
