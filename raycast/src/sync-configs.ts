import { showToast, Toast } from "@raycast/api";

// Placeholder until the CLI wiring lands: real implementation runs
// `agentcfg sync` and reports the summary line.
export default async function main() {
  await showToast({
    style: Toast.Style.Failure,
    title: "Not implemented yet",
    message: "agentcfg sync wiring lands in the next phase",
  });
}
