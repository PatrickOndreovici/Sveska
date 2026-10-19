import { Eyebrow } from "../components/ui/Eyebrow";
import { TextLink } from "../components/ui/TextLink";

export function NotFoundPage() {
  return (
    <article className="max-w-prose">
      <Eyebrow>Not found</Eyebrow>
      <h1 className="mt-3 font-serif text-4xl text-ink">This page does not exist.</h1>
      <p className="mt-6 text-ink-muted">
        Check the address, or <TextLink to="/">return to the workspace</TextLink>.
      </p>
    </article>
  );
}
