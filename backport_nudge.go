package blathers

var nudge = `Thanks for opening a [backport](https://cockroachlabs.atlassian.net/wiki/spaces/CRDB/pages/900005932/Backporting+a+change+to+a+release+branch).

Please check the backport criteria before merging:

- [ ] Patches should only be created for serious issues.
- [ ] Patches should not break backwards-compatibility.
- [ ] Patches should change as little code as possible.
- [ ] Patches should not change on-disk formats or node communication protocols.
- [ ] Patches should not add new functionality.


<details>
  <summary>If some of the basic criteria cannot be satisfied, ensure that the exceptional criteria are satisfied within.</summary>

- [ ] There is a high priority need for the functionality that cannot wait until the next release and is difficult to address in another way.
- [ ] The new functionality is additive-only and only runs for clusters which have specifically “opted in” to it (e.g. by a cluster setting).
- [ ] New code is protected by a conditional check that is trivial to verify and ensures that it only runs for opt-in clusters.
- [ ] The PM and TL on the team that owns the changed code have signed off that the change obeys the above rules.
</details>

Add a brief release justification to the body of your PR to justify this backport.

Some other things to consider:

- What did we do to ensure that a user that doesn’t know & care about this backport, has no idea that it happened?
- Will this work in a cluster of mixed patch versions? Did we test that?
- If a user upgrades a patch version, uses this feature, and then downgrades, what happens?`
