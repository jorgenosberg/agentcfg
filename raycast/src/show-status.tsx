import { Action, ActionPanel, Color, Icon, List, Keyboard } from "@raycast/api";
import { useExec } from "@raycast/utils";
import { parseJsonOutput, resolveBinary, type StatusEntry } from "./lib/agentcfg";

const STATUS_ICONS: Record<string, { source: Icon; tintColor: Color }> = {
  linked: { source: Icon.Link, tintColor: Color.Green },
  copied: { source: Icon.CheckCircle, tintColor: Color.Green },
  drifted: { source: Icon.ExclamationMark, tintColor: Color.Orange },
  unmanaged: { source: Icon.QuestionMarkCircle, tintColor: Color.Yellow },
  absent: { source: Icon.Circle, tintColor: Color.SecondaryText },
  disabled: { source: Icon.MinusCircle, tintColor: Color.SecondaryText },
  "plugin-owned": { source: Icon.Plug, tintColor: Color.Blue },
  "plugin-sibling": { source: Icon.Plug, tintColor: Color.SecondaryText },
  "n/a": { source: Icon.Minus, tintColor: Color.SecondaryText },
};

export default function ShowStatus() {
  const bin = resolveBinary();
  const { data, isLoading, revalidate } = useExec(bin ?? "", ["status", "--json"], {
    execute: !!bin,
    parseOutput: (out) => parseJsonOutput<StatusEntry[]>(out),
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

  const targets = [...new Set(data?.map((e) => e.target))];

  return (
    <List isLoading={isLoading} searchBarPlaceholder="Search status entries…">
      {targets.map((target) => (
        <List.Section key={target} title={target}>
          {data
            ?.filter((e) => e.target === target)
            .map((entry) => (
              <List.Item
                key={`${entry.target}/${entry.kind}/${entry.item}`}
                icon={STATUS_ICONS[entry.status] ?? Icon.Dot}
                title={entry.item}
                subtitle={entry.dest}
                accessories={[
                  ...(entry.plugin ? [{ tag: { value: entry.plugin, color: Color.Blue } }] : []),
                  { tag: entry.kind },
                  { text: entry.status },
                ]}
                actions={
                  <ActionPanel>
                    <Action.ShowInFinder path={entry.dest} />
                    <Action.CopyToClipboard
                      title="Copy Destination Path"
                      content={entry.dest}
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
