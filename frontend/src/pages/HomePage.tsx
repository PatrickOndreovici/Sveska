import { Eyebrow } from "../components/ui/Eyebrow";

export function HomePage() {
  return (
    <article className="max-w-prose">
      <Eyebrow>Workspace</Eyebrow>
      <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">
        A quiet place to read, annotate and practise Serbian.
      </h1>
      <p className="mt-6 font-serif text-lg leading-relaxed text-ink-muted">
        Lessons, vocabulary and teacher feedback will live here. This build contains the project
        foundation only; no learning features are available yet.
      </p>
      <hr className="mt-10 w-16 border-t-2 border-accent" />
      <p lang="sr" className="mt-6 font-serif italic text-ink-muted">
        Čitaj, piši, ponavljaj. · Читај, пиши, понављај.
      </p>
    </article>
  );
}
