import { ExternalLink } from "lucide-react";
import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown, { type Components, defaultUrlTransform } from "react-markdown";
import { Link } from "react-router";
import remarkGfm from "remark-gfm";

import { cn } from "@/lib/utils";

/**
 * Only https/mailto, in-page anchors and app routes survive; everything else (javascript:, data:, http:, //host)
 * becomes an empty href. Raw HTML in the Markdown is never parsed as HTML (no rehype-raw): it renders as text.
 */
export function safeUrl(url: string): string {
  if (url.startsWith("/") && !url.startsWith("//")) return url;
  if (url.startsWith("#")) return url;
  if (/^(https:\/\/|mailto:)/i.test(url)) return defaultUrlTransform(url);
  return "";
}

// External links open in a new tab without access to this window (no opener, no referrer).
const externalAttrs = { target: "_blank", rel: "noopener noreferrer" } as const;

function MarkdownLink({ href = "", children, ...rest }: ComponentProps<"a">) {
  const { t } = useTranslation("help");
  if (href.startsWith("/")) {
    return (
      <Link to={href} className="text-link underline underline-offset-4">
        {children}
      </Link>
    );
  }
  if (href === "") return <span>{children}</span>;
  const external = !href.startsWith("#");
  return (
    <a
      {...rest}
      href={href}
      className="text-link underline underline-offset-4"
      {...(external ? externalAttrs : {})}
    >
      {children}
      {external && (
        <>
          <ExternalLink className="ml-0.5 inline size-3.5 align-baseline" aria-hidden />
          <span className="sr-only">{t("newTab")}</span>
        </>
      )}
    </a>
  );
}

type Tag = "h2" | "h3" | "h4" | "p" | "ul" | "ol" | "blockquote" | "code" | "pre" | "th" | "td";

/** Element with a base class; drops react-markdown's `node` prop so it never reaches the DOM. */
function styled<T extends Tag>(As: T, base: string) {
  return function Styled({ node, className, ...rest }: ComponentProps<T> & { node?: unknown }) {
    void node;
    const Comp = As as React.ElementType;
    return <Comp className={cn(base, className)} {...rest} />;
  };
}

const Table = ({ node, className, ...rest }: ComponentProps<"table"> & { node?: unknown }) => {
  void node;
  return (
    <div className="mt-4 overflow-x-auto">
      <table className={cn("w-full border-collapse text-sm", className)} {...rest} />
    </div>
  );
};

// Markdown h1 is the page title inside the article; the shell header owns the document's only <h1>.
const components: Components = {
  a: ({ node, ...props }) => {
    void node;
    return <MarkdownLink {...props} />;
  },
  h1: styled("h2", "mt-2 text-2xl font-semibold tracking-tight"),
  h2: styled("h3", "mt-8 border-b pb-1 text-xl font-semibold"),
  h3: styled("h4", "mt-6 text-lg font-semibold"),
  p: styled("p", "mt-3 leading-7"),
  ul: styled("ul", "mt-3 list-disc space-y-1 pl-6"),
  ol: styled("ol", "mt-3 list-decimal space-y-1 pl-6"),
  blockquote: styled("blockquote", "mt-4 rounded-r-md border-l-4 border-info bg-info-bg px-4 py-2 text-info-fg"),
  code: styled("code", "rounded bg-muted px-1 py-0.5 font-mono text-[0.9em]"),
  pre: styled("pre", "mt-4 overflow-x-auto rounded-md bg-muted p-3 font-mono text-sm [&>code]:bg-transparent [&>code]:p-0"),
  table: Table,
  th: styled("th", "border-b px-3 py-2 text-left font-medium"),
  td: styled("td", "border-b px-3 py-2 align-top"),
};

// Compact variant for short texts such as OpenAPI descriptions: no headings, paragraphs without top margin.
const compactComponents: Components = {
  ...components,
  p: styled("p", "leading-6 [&+p]:mt-2"),
};

/** Help Markdown (GFM) renderer: no raw HTML, no images, safe links. */
export function Markdown({ children, compact = false }: { children: string; compact?: boolean }) {
  return (
    <div
      className={compact ? "text-sm text-muted-foreground" : "max-w-3xl text-foreground"}
      data-testid={compact ? undefined : "help-markdown"}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={compact ? compactComponents : components}
        urlTransform={safeUrl}
        disallowedElements={compact ? ["img", "h1", "h2", "h3", "h4", "h5", "h6"] : ["img"]}
        unwrapDisallowed={compact}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
