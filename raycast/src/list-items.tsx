import { Action, ActionPanel, Icon, List, showToast, Toast, Keyboard } from "@raycast/api";
import { showFailureToast, useExec } from "@raycast/utils";
import { parseJsonOutput, resolveBinary, runAgentcfg, type Item } from "./lib/agentcfg";

const KIND_ICONS: Record<string, Icon> = {
  skill: Icon.Stars,
  hook: Icon.Bolt,
  context: Icon.Document,
  command: Icon.Terminal,
  rule: Icon.CheckList,
};

const KIND_ORDER = ["skill", "hook", "context", "command", "rule"];

export default function ListItems() {
  const bin = resolveBinary();
  const { data, isLoading, revalidate } = useExec(bin ?? "", ["list", "--json"], {
    execute: !!bin,
    parseOutput: (out) => parseJsonOutput<Item[]>(out),
  });

  if (!bin) {
    return (
      <List>
        <List.EmptyView
          icon={Icon.Warning}
          title="agentcfg binary not found"
          description="Install it with brew, or set its path in the extension preferences (⌘,)."
        />
      </List>
    );
  }

  async function toggle(item: Item) {
    try {
      const out = await runAgentcfg(["toggle", item.name]);
      await showToast({ style: Toast.Style.Success, title: `Toggled ${item.name}`, message: out.trim() });
      revalidate();
    } catch (error) {
      await showFailureToast(error, { title: `Could not toggle ${item.name}` });
    }
  }

  const kinds = KIND_ORDER.filter((k) => data?.some((it) => it.kind === k));

  return (
    <List isLoading={isLoading} searchBarPlaceholder="Search items…">
      {kinds.map((kind) => (
        <List.Section key={kind} title={`${kind}s`}>
          {data
            ?.filter((it) => it.kind === kind)
            .map((item) => (
              <List.Item
                key={item.path}
                icon={KIND_ICONS[item.kind] ?? Icon.Dot}
                title={item.name}
                subtitle={item.path}
                actions={
                  <ActionPanel>
                    <Action.Open title="Open in Editor" target={item.path} />
                    <Action icon={Icon.Switch} title="Toggle Item" onAction={() => toggle(item)} />
                    <Action.ShowInFinder path={item.path} shortcut={{ modifiers: ["cmd"], key: "f" }} />
                    <Action.CopyToClipboard
                      title="Copy Path"
                      content={item.path}
                      shortcut={{ modifiers: ["cmd"], key: "c" }}
                    />
                    <Action
                      icon={Icon.ArrowClockwise}
                      title="Refresh"
                      onAction={revalidate}
                      shortcut={Keyboard.Shortcut.Common.Refresh}
                    />
                  </ActionPanel>
                }
              />
            ))}
        </List.Section>
      ))}
    </List>
  );
}
