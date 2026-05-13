<!-- Thanks for contributing to Forge Engineering Fabric. -->

## Summary

<!-- One paragraph: what does this PR do and why? -->

## Scope

- [ ] Code change
- [ ] Documentation change
- [ ] Specification change (OpenSpec)
- [ ] Infrastructure / deploy change

## Linked work

<!-- Link to OpenSpec change folder, issue, or design doc. -->

## Test plan

<!-- How was this verified? Include commands run, screenshots, etc. -->

## Portal checklist

If this PR touches `portal/src/`, confirm all of the following before requesting review:

- [ ] No mocks, fixtures, fake data, demo names or imports from `design/` in shipped Portal source (`make audit-no-mocks` passes locally)
- [ ] Every visible string comes from `portal/src/i18n/dictionary.ts` (no hard-coded English/Spanish)
- [ ] Tokens — no raw hex/rgb/hsl colours; surfaces use design-system classes
- [ ] Theme works in `light`, `dark` and `system`
- [ ] Accessibility — focus rings visible, `@axe-core/playwright` zero serious violations
- [ ] Visual baselines (`portal/tests/visual/__screenshots__/`) regenerated and approved
- [ ] Server-bound data only — every prop traces back to a real platform endpoint

## Security checklist

- [ ] No secrets committed
- [ ] No new external network egress without review
- [ ] OPA policies updated if a new action is introduced
