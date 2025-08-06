import { LumenWiki } from "./LumenWiki";

export function LumenWikiExample() {
  return (
    <div className="container mx-auto py-8">
      {/* Single LumenWiki instance - AI responds when info button is clicked */}
      <LumenWiki request="What is Apple" />
    </div>
  );
}
