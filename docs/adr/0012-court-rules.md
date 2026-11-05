# 12. Court rules steer which matches each court takes

- Status: accepted, amends ADR 6
- Date: 2026-11-05

## Context

ADR 6 shares the courts between two categories in two ways: *in turn*, or
*reserved courts* per category until it is over. Organizers want finer
control, for reasons of the hall: the women's doubles pools on courts 1 and
2 while they last, pools A and B of the men on courts 3 and 4, court 7 kept
for the brackets or the final, a court never used by a category. Several
courts may share the same wishes, and a wish stops mattering once its
matches are played.

## Decision

- **Each court has rules.** A rule targets a *scope*: a category, a stage of
  it, a pool of a pools stage ("A"), or a round of a bracket stage, named by
  its size and the first place it plays for (2 and 1: the final; 2 and 3:
  the third place; 4 and 5: places 5 to 8), so a rule keeps its meaning
  whatever the bracket's size. A court has at most one rule per scope.
- Three kinds:
  - **priority**, with a level from 1 (first) to 9: when the court frees up,
    it takes the ready match of its highest-priority scope, **before** the
    matches ahead of it in the schedule order; at the same level, and
    between matches of no priority, the order decides. Other courts still
    take these matches in order: a priority is not an exclusivity;
  - **only**: the court takes nothing else while one of its only scopes has
    matches left, and stays idle meanwhile;
  - **never**: the forecast and the proposals never put a match of the scope
    on the court.
- **A rule ends with its scope**: once no match of it is left to place
  (stand-ins of stages not filled yet included), it no longer applies. An
  only court then takes anything.
- Among courts free at the same time, a court with an only rule running
  goes first, then one with a priority running: the courts set for a scope
  get its matches (the final on court 7, not on court 1 free at the same
  minute).
- **Never block the user.** Rules drive the forecast and therefore the
  proposals. Calling a match by hand on a court whose rules keep it off is
  allowed; the queue marks those courts. When the rules leave matches that
  no court may ever take, the forecast drops them and places those matches
  anyway, flagged (`OffRules`), and the schedule warns.
- **Combination with the modes**: *in turn* still sets the order; rules set
  the courts. In *reserved* mode, the courts without an only rule share the
  categories as before. Courts reserved by hand in older files read as an
  only rule for their category; in alternate mode they were ignored and stay
  so.
- **Storage**: the rules are part of the `ScheduleSet` payload (`rules`:
  court → list), so each change is one undoable event in the history, as the
  other schedule settings. Rules naming a removed category, stage or court
  are left out on read.
- **Pure forecast**: `forecast.Input.Rules` holds them; `forecast.Run`
  applies them in `pick` (filter, then the best level), counts the items
  left per rule to end it, and revives idle courts after each placement so
  an only court sees its scope become ready.
- **UI**: a foldable "Court rules" card on the Schedule screen lists each
  court's rules (greyed once over) and adds one to several courts at once:
  courts, kind, level, scope (category → stage → pool or round). While a
  rule is being chosen, the timeline previews it (`PreviewCourtRule`,
  projected in memory).

## Consequences

- One court model covers hall zones, reserved courts, a show court for the
  final, and courts kept away from a category.
- A priority reorders matches across categories: a court may start a
  later round of its scope before an earlier block of the order. The order
  still rules on the courts without priorities, and within each scope.
- The rules are checked once per placement over the courts' rules: a few
  rules per court keep the forecast fast.
