"use client";

import { useEffect, useState } from "react";
import { useLang } from "@/components/providers/LangProvider";
import { useSession } from "next-auth/react";

export function Greeting() {
  const { t } = useLang();
  const { data: session } = useSession();
  const [hour, setHour] = useState<number | null>(null);

  useEffect(() => {
    setHour(new Date().getHours());
  }, []);

  const key =
    hour == null
      ? "h_hello"
      : hour >= 18 || hour < 5
        ? "h_hello_n"
        : hour >= 12
          ? "h_hello_pm"
          : "h_hello";
  const name = (session?.user?.name ?? session?.user?.email ?? "").split(/[\s@]/)[0] || "—";
  return (
    <div className="h-eyebrow" style={{ marginBottom: 8 }}>
      {t(key)} {name.toUpperCase()}
    </div>
  );
}
