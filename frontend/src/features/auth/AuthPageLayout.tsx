import type { ReactNode } from "react";

export function AuthPageLayout({ title, intro, children }: { title: string; intro: string; children: ReactNode }) {
  return (
    <section className="mx-auto grid w-full max-w-md gap-8">
      <header className="grid gap-3">
        <h1 className="font-serif text-4xl text-ink">{title}</h1>
        <p className="text-ink-muted">{intro}</p>
      </header>
      {children}
    </section>
  );
}
