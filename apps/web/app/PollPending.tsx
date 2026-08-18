"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

/** 派生画像待ちのあいだ server コンポーネントを再取得する。ブラウザから API を直接叩かない。 */
export function PollPending({ active }: { active: boolean }) {
  const router = useRouter();
  useEffect(() => {
    if (!active) {
      return;
    }
    const id = setInterval(() => router.refresh(), 2500);
    return () => clearInterval(id);
  }, [active, router]);
  return null;
}
