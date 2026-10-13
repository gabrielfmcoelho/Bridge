"use client";

import Drawer from "@/components/ui/Drawer";
import Icon from "@/components/ui/Icon";
import IconButton from "@/components/ui/IconButton";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import { AiUnavailable, ChatComposer, ChatSuggestions, ChatThread, useAiStatus } from "./Chat";
import { useAiChat } from "./chatStore";

const SUGGESTIONS = ["ai.chat.suggest.summary", "ai.chat.suggest.disk", "ai.chat.suggest.dns"];

/** The header's assistant: the shared thread (chatStore), full height. Open
 *  state lives in the store, so the home widget can open it too. */
export default function AiChatDrawer() {
  const { t } = useLocale();
  const chat = useAiChat();
  const status = useAiStatus();
  const unavailable = !status.loading && !status.ready;
  return (
    <Drawer
      open={chat.drawerOpen}
      onClose={chat.close}
      title={t("ui.aiChatDrawer.title")}
      headerAction={!unavailable && chat.messages.length > 0 && (
        <IconButton label={t("ai.chat.newChat")} onClick={chat.clear}>
          <Icon path={ICON_PATHS.plus} className="w-4 h-4" />
        </IconButton>
      )}
      footer={unavailable ? undefined : <ChatComposer pending={chat.pending} onSend={chat.send} onStop={chat.stop} autoFocus={chat.drawerOpen} />}
    >
      {unavailable ? <AiUnavailable disabled={status.disabled} /> : <ChatThread
        messages={chat.messages}
        pending={chat.pending}
        onRetry={chat.retry}
        className="h-full min-h-[50vh]"
        empty={
          <div className="space-y-3 py-6">
            <p className="text-sm text-[var(--text-muted)]">{t("ui.aiChatDrawer.emptyHint")}</p>
            <ChatSuggestions prompts={SUGGESTIONS.map((k) => t(k))} onPick={chat.send} />
          </div>
        }
      />}
    </Drawer>
  );
}
