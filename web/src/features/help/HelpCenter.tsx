import { BookOpen, FileQuestion, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, NavLink, useParams } from "react-router";

import { StatePage } from "@/app/pages/StatePage";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

import { API_REFERENCE_PATH, HELP_HOME, type HelpMeta, useHelpIndex, useHelpPage } from "./api";
import { ApiReference } from "./ApiReference";
import { Markdown } from "./Markdown";
import { searchHelp } from "./search";

const pagePath = (slug: string) => (slug === HELP_HOME ? "/help" : `/help/${slug}`);

function PageList({ pages }: { pages: HelpMeta[] }) {
  const { t } = useTranslation("help");
  const [query, setQuery] = useState("");
  const hits = useMemo(() => searchHelp(pages, query), [pages, query]);
  const audiences = ["user", "admin"] as const;

  return (
    <div className="space-y-3">
      <div className="relative">
        <Search className="pointer-events-none absolute top-2.5 left-2.5 size-4 text-muted-foreground" aria-hidden />
        <Input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("searchPlaceholder")}
          aria-label={t("searchLabel")}
          className="pl-8"
          data-testid="help-search"
        />
      </div>
      {query.trim() !== "" ? (
        <ul className="space-y-0.5" data-testid="help-results" aria-live="polite">
          {hits.length === 0 && <li className="px-2 text-sm text-muted-foreground">{t("noResults")}</li>}
          {hits.map(({ page, heading }) => (
            <li key={page.slug}>
              <Link to={pagePath(page.slug)} className="block rounded-md px-2 py-1.5 text-sm hover:bg-accent">
                <span className="font-medium">{page.title}</span>
                {heading && <span className="block text-xs text-muted-foreground">{heading}</span>}
              </Link>
            </li>
          ))}
        </ul>
      ) : (
        audiences.map((aud) => {
          const list = pages.filter((p) => p.audience === aud);
          if (list.length === 0) return null;
          return (
            <div key={aud} role="group" aria-labelledby={`help-aud-${aud}`}>
              <p id={`help-aud-${aud}`} className="px-2 pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                {t(`audience.${aud}`)}
              </p>
              <ul className="space-y-0.5">
                {list.map((p) => (
                  <li key={p.slug}>
                    <NavLink
                      to={pagePath(p.slug)}
                      end
                      data-help={p.slug}
                      className={({ isActive }) =>
                        cn(
                          "block rounded-md px-2 py-1.5 text-sm",
                          isActive ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:bg-accent",
                        )
                      }
                    >
                      {p.title}
                    </NavLink>
                  </li>
                ))}
                {aud === "admin" && (
                  <li>
                    <NavLink
                      to={API_REFERENCE_PATH}
                      data-help="api-reference"
                      className={({ isActive }) =>
                        cn(
                          "flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm",
                          isActive ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:bg-accent",
                        )
                      }
                    >
                      <BookOpen className="size-3.5" aria-hidden />
                      {t("api.title")}
                    </NavLink>
                  </li>
                )}
              </ul>
            </div>
          );
        })
      )}
    </div>
  );
}

function PageContent({ slug }: { slug: string }) {
  const { t } = useTranslation("help");
  const page = useHelpPage(slug);
  if (page.isPending) {
    return (
      <div className="space-y-3" role="status" aria-label={t("loading")}>
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
      </div>
    );
  }
  if (page.isError) {
    return <StatePage testId="help-not-found" icon={FileQuestion} title={t("notFoundTitle")} body={t("notFoundBody")} />;
  }
  return (
    <article lang={page.data.locale} data-slug={page.data.slug}>
      <Markdown>{page.data.markdown}</Markdown>
    </article>
  );
}

/** Help Center module (/help/*): page list + search on the left, content on the right. */
export function HelpCenter() {
  const { t } = useTranslation("help");
  const rest = useParams()["*"] ?? "";
  const slug = rest.replace(/\/+$/, "") || HELP_HOME;
  const index = useHelpIndex();

  return (
    <div className="flex flex-col gap-6 lg:flex-row" data-testid="help-center">
      <nav aria-label={t("pagesNav")} className="shrink-0 lg:w-64">
        {index.isPending ? <Skeleton className="h-40 w-full" /> : index.isError ? <p role="alert">{t("loadError")}</p> : <PageList pages={index.data.pages} />}
      </nav>
      <div className="min-w-0 flex-1">{`/help/${slug}` === API_REFERENCE_PATH ? <ApiReference /> : <PageContent slug={slug} />}</div>
    </div>
  );
}
