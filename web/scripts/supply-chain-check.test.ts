import { readFileSync } from "node:fs";
import { join } from "node:path";

import { findPolicyViolations, POLICY_FILES } from "./supply-chain-check.ts";

describe("supply:check (ISS-031)", () => {
  it("accepts the repository's current policy files", () => {
    const files = Object.fromEntries(
      POLICY_FILES.flatMap((f) => {
        try {
          return [[f, readFileSync(join(process.cwd(), f), "utf8")]];
        } catch {
          return [];
        }
      }),
    );
    expect(findPolicyViolations(files)).toEqual([]);
  });

  it.each([
    ["pnpm-workspace.yaml", "minimumReleaseAgeExclude:\n  - lucide-react@1.56.0\n"],
    ["pnpm-workspace.yaml", "# note\n  minimumReleaseAgeExclude: ['radix-ui']\n"],
    [".npmrc", "minimum-release-age-exclude=radix-ui\n"],
    ["package.json", '{ "pnpm": { "minimumReleaseAgeExclude": ["sonner"] } }'],
    ["pnpm-workspace.yaml", "minimumReleaseAgeIgnoreMissingTime: true\n"],
  ])("rejects an exclusion in %s", (name, text) => {
    expect(findPolicyViolations({ [name]: text })).toHaveLength(1);
  });

  it.each([
    ["pnpm-workspace.yaml", "minimumReleaseAge: 0\n"],
    ["pnpm-workspace.yaml", "minimumReleaseAge: 60\n"],
    [".npmrc", "minimum-release-age=1\n"],
    ["package.json", '{ "pnpm": { "minimumReleaseAge": 10 } }'],
  ])("rejects a lowered release age in %s", (name, text) => {
    expect(findPolicyViolations({ [name]: text })).toHaveLength(1);
  });

  it("allows a stricter release age and comments that mention the policy", () => {
    expect(
      findPolicyViolations({
        "pnpm-workspace.yaml": "# never add minimumReleaseAgeExclude entries\nminimumReleaseAge: 4320\n",
      }),
    ).toEqual([]);
  });
});
