# Open Source Code Signing Adoption — Governance Record

**Project**: libvalidate — widely-used open source input validation library
**Event period**: 2026-02-01 to 2026-03-15
**Document type**: Composite record (emergency meeting notes + proposal text + vote record + ratification notice + implementation log + dissent registry)

---

## Span 1 — Emergency Meeting (2026-02-01)

Following the supply chain compromise of npm package `strutil` v2.1.3, which distributed a backdoored release to 84,000 downstream consumers, the libvalidate registry-security-team convened an emergency meeting. The team declared a Code Signing Readiness Review and recommended that mandatory package-signing enforcement be adopted within 30 days.

---

## Span 2 — Working Group Formation (2026-02-03)

The board-security-committee, acting under foundation-committee-charter article 12.4, formed a governance-working-group with a 14-day mandate to draft a mandatory code-signing policy. The working group was granted authority to consult external signatories and propose enforcement mechanisms to the full community.

---

## Span 3 — Policy Proposal Published (2026-02-10)

The governance-working-group published PROPOSAL-CSRP-001: Mandatory Code Signing Requirement. The proposal required all new releases to be signed with a GPG key registered to a verified account. Releases published before the enforcement date were grandfathered. Implementation deadline: 2026-03-15.

---

## Span 4 — Community Vote (2026-02-17)

PROPOSAL-CSRP-001 passed the community vote: 47 in favor, 5 opposed, 0 abstentions, out of 52 eligible voters. The vote was conducted under the community-governance-process and was declared binding per registry-governance-rules section 8 (simple majority required for security policy changes).

---

## Span 5 — Board Ratification (2026-02-21)

Foundation board formally ratified PROPOSAL-CSRP-001 under foundation-bylaws-article-7 (emergency policy adoption). The ratification elevated the policy from community-advisory status to a legally-binding obligation for all packages listed in the libvalidate registry.

---

## Span 6 — Registry Enforcement Deployed (2026-03-15)

The package-signing-gate was deployed to the registry's release pipeline. All new release attempts without a valid registered GPG signature were rejected with error code `SIGN-001`. Existing releases were not removed. A pre-policy snapshot was taken; 847 packages were grandfathered under the snapshot.

---

## Span 7 — Formal Dissent Registry (2026-02-25)

Three core maintainers — primarily operating from low-resource computing environments — formally registered objections under registry-governance-rules section 9 (formal objection procedure). The objection stated that GPG key infrastructure requirements created a barrier for contributors without access to persistent signing environments, effectively translating the policy into an exclusion of a class of contributors from the release process.
