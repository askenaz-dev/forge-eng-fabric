"use client";

import * as Popover from "@radix-ui/react-popover";
import { useEffect, useState } from "react";
import { Bell } from "../icons";
import { useLang } from "../providers/LangProvider";

type NotificationEvent = {
  id: string;
  type: string;
  text: string;
  when: string;
  href?: string;
};

export function NotificationsButton() {
  const { t } = useLang();
  const [unread, setUnread] = useState<number>(0);
  const [items, setItems] = useState<NotificationEvent[]>([]);

  useEffect(() => {
    let es: EventSource | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let dead = false;

    function connect() {
      try {
        es = new EventSource("/api/notifications/stream");
        es.onmessage = (ev) => {
          try {
            const data = JSON.parse(ev.data) as NotificationEvent;
            setItems((cur) => [data, ...cur].slice(0, 25));
            setUnread((u) => u + 1);
          } catch {
            // ignored malformed payload
          }
        };
        es.onerror = () => {
          es?.close();
          if (!dead) timer = setTimeout(connect, 5000);
        };
      } catch {
        // SSE may be unsupported (rare); we degrade silently and rely on
        // page refreshes / router.refresh() to re-pull.
      }
    }
    connect();
    return () => {
      dead = true;
      es?.close();
      if (timer) clearTimeout(timer);
    };
  }, []);

  function acknowledge() {
    if (unread === 0) return;
    setUnread(0);
    fetch("/api/notifications/ack", { method: "POST", keepalive: true }).catch(() => undefined);
  }

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <button className="icon-btn" aria-label={t("tb_notif")} onClick={acknowledge}>
          <Bell />
          {unread > 0 && <span className="dot" aria-label={`${unread} ${t("tb_notif")}`} />}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="pop" style={{ minWidth: 280 }} align="end" sideOffset={8} collisionPadding={12}>
          {items.length === 0 ? (
            <div className="pop-item" aria-disabled>
              <small style={{ marginLeft: 0 }}>{t("act_sub")}</small>
            </div>
          ) : (
            items.map((it) => (
              <a key={it.id} href={it.href ?? "/incidents"} className="pop-item">
                <span>{it.text}</span>
                <small>{it.when}</small>
              </a>
            ))
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
