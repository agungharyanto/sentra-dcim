import { ChevronRight } from "lucide-react";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { CopyButton } from "@/components/domain/CopyButton";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

import {
  type HttpMethod,
  type OpenApiDoc,
  type OpenApiOperation,
  type OpenApiParameter,
  type OpenApiResponse,
  type OpenApiSchema,
  useOpenApi,
} from "./api";
import { Markdown } from "./Markdown";

const METHODS: readonly HttpMethod[] = ["get", "post", "put", "patch", "delete", "head", "options"];

/** Resolves a local "#/components/<kind>/<name>" reference. */
export function resolveRef<T>(doc: OpenApiDoc, ref: string): T | undefined {
  const parts = ref.replace(/^#\//, "").split("/");
  let cur: unknown = doc;
  for (const p of parts) cur = (cur as Record<string, unknown> | undefined)?.[p];
  return cur as T | undefined;
}

const refName = (ref: string) => ref.split("/").pop() ?? ref;

/** Operations grouped by their first tag, in the order of the spec's tag list. */
export function groupOperations(doc: OpenApiDoc) {
  const groups = new Map<string, { method: string; path: string; op: OpenApiOperation }[]>();
  for (const t of doc.tags ?? []) groups.set(t.name, []);
  for (const [path, item] of Object.entries(doc.paths)) {
    for (const method of METHODS) {
      const found = item[method];
      if (!found) continue;
      // Path-level parameters (e.g. {id}) apply to every operation of the path.
      const op = item.parameters ? { ...found, parameters: [...item.parameters, ...(found.parameters ?? [])] } : found;
      const tag = op.tags?.[0] ?? "other";
      if (!groups.has(tag)) groups.set(tag, []);
      groups.get(tag)?.push({ method, path, op });
    }
  }
  return [...groups.entries()].filter(([, ops]) => ops.length > 0);
}

function typeLabel(s: OpenApiSchema): string {
  if (s.$ref) return refName(s.$ref);
  const t = Array.isArray(s.type) ? s.type.join(" | ") : s.type;
  if (t === "array" && s.items) return `${typeLabel(s.items)}[]`;
  return [t ?? (s.properties ? "object" : "any"), s.format && `(${s.format})`].filter(Boolean).join(" ");
}

/** Read-only schema tree with $ref resolution and a cycle guard. */
function SchemaView({ doc, schema, seen = [] }: { doc: OpenApiDoc; schema: OpenApiSchema; seen?: string[] }) {
  const { t } = useTranslation("help");
  if (schema.$ref) {
    if (seen.includes(schema.$ref)) return <code className="font-mono text-xs">{refName(schema.$ref)}</code>;
    const target = resolveRef<OpenApiSchema>(doc, schema.$ref);
    return target ? <SchemaView doc={doc} schema={target} seen={[...seen, schema.$ref]} /> : null;
  }
  const composed = schema.allOf ?? schema.oneOf ?? schema.anyOf;
  if (composed) {
    return (
      <div className="space-y-2">
        {schema.oneOf && <p className="text-xs text-muted-foreground">{t("api.oneOf")}</p>}
        {composed.map((s, i) => (
          <SchemaView key={i} doc={doc} schema={s} seen={seen} />
        ))}
      </div>
    );
  }
  const item = schema.type === "array" && schema.items ? schema.items : undefined;
  const props = schema.properties ?? (item?.$ref ? resolveRef<OpenApiSchema>(doc, item.$ref)?.properties : undefined);
  return (
    <div className="space-y-1 text-sm">
      <p>
        <code className="font-mono text-xs">{typeLabel(schema)}</code>
        {schema.enum && <span className="ml-2 font-mono text-xs text-muted-foreground">{schema.enum.map(String).join(" | ")}</span>}
      </p>
      {schema.description && <Markdown compact>{schema.description}</Markdown>}
      {props && (
        <ul className="space-y-2 border-l pl-3">
          {Object.entries(props).map(([name, p]) => {
            const required = (schema.required ?? []).includes(name);
            return (
              <li key={name}>
                <span className="font-mono text-xs font-medium">{name}</span>
                {required && <span className="ml-1 text-xs text-status-critical-fg">{t("api.required")}</span>}
                <div className="mt-0.5">
                  <SchemaView doc={doc} schema={p} seen={seen} />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

function Parameters({ doc, params }: { doc: OpenApiDoc; params: OpenApiParameter[] }) {
  const { t } = useTranslation("help");
  const resolved = params.map((p) => (p.$ref ? resolveRef<OpenApiParameter>(doc, p.$ref) : p)).filter((p): p is OpenApiParameter => !!p);
  if (resolved.length === 0) return <p className="text-sm text-muted-foreground">{t("api.noParameters")}</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left">
            <th className="py-1 pr-3 font-medium">{t("api.name")}</th>
            <th className="py-1 pr-3 font-medium">{t("api.in")}</th>
            <th className="py-1 pr-3 font-medium">{t("api.type")}</th>
            <th className="py-1 font-medium">{t("api.description")}</th>
          </tr>
        </thead>
        <tbody>
          {resolved.map((p) => (
            <tr key={`${p.in}:${p.name}`} className="border-b align-top">
              <td className="py-1 pr-3 font-mono text-xs">
                {p.name}
                {p.required && <span className="ml-1 text-status-critical-fg">{t("api.required")}</span>}
              </td>
              <td className="py-1 pr-3">{p.in}</td>
              <td className="py-1 pr-3 font-mono text-xs">
                {p.schema ? typeLabel(p.schema) : null}
                {p.schema?.enum && <span className="block text-muted-foreground">{p.schema.enum.map(String).join(" | ")}</span>}
              </td>
              <td className="py-1">{p.description && <Markdown compact>{p.description}</Markdown>}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Responses({ doc, responses }: { doc: OpenApiDoc; responses: Record<string, OpenApiResponse> }) {
  return (
    <ul className="space-y-3">
      {Object.entries(responses).map(([code, raw]) => {
        const r = raw.$ref ? (resolveRef<OpenApiResponse>(doc, raw.$ref) ?? raw) : raw;
        const schema = r.content && Object.values(r.content)[0]?.schema;
        return (
          <li key={code} data-status-code={code}>
            <div className="flex items-start gap-2">
              <code className={cn("shrink-0 rounded px-1.5 py-0.5 font-mono text-xs", code.startsWith("2") ? "bg-menu-available-bg text-menu-available-fg" : "bg-muted")}>
                {code}
              </code>
              {r.description && <Markdown compact>{r.description}</Markdown>}
            </div>
            {schema && (
              <div className="mt-1 ml-2">
                <SchemaView doc={doc} schema={schema} />
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

export type Access = "public" | "internal" | "login";

/** Access level from the spec's x-sentra-access extension (never hardcoded per path in the UI). */
export function accessOf(op: OpenApiOperation): Access | undefined {
  const v = op["x-sentra-access"];
  return v === "public" || v === "internal" || v === "login" ? v : undefined;
}

/**
 * One endpoint as a disclosure. Header structure avoids nested interactive elements: the toggle <button> holds
 * method + path and stretches over the whole row (::after), the copy button is a sibling placed above it (z-10).
 */
function Operation({ doc, method, path, op }: { doc: OpenApiDoc; method: string; path: string; op: OpenApiOperation }) {
  const { t } = useTranslation("help");
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const body = op.requestBody?.content && Object.values(op.requestBody.content)[0]?.schema;
  const access = accessOf(op);
  return (
    <div className="rounded-lg border bg-card" data-operation={`${method.toUpperCase()} ${path}`} data-open={open}>
      <div className="group relative flex flex-wrap items-center gap-2 rounded-lg px-4 py-2.5 hover:bg-accent/50">
        <button
          type="button"
          aria-expanded={open}
          aria-controls={panelId}
          onClick={() => setOpen(!open)}
          className="flex items-center gap-2 text-left outline-none after:absolute after:inset-0 after:rounded-lg after:content-[''] focus-visible:after:ring-[3px] focus-visible:after:ring-ring"
          data-testid="operation-toggle"
        >
          <ChevronRight className={cn("size-4 shrink-0 transition-transform", open && "rotate-90")} aria-hidden />
          <span className="rounded bg-primary px-2 py-0.5 font-mono text-xs font-semibold text-primary-foreground uppercase">{method}</span>
          <code className="font-mono text-sm break-all">{path}</code>
        </button>
        <CopyButton
          value={path}
          label={t("api.copyPath", { path })}
          className="relative z-10 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 data-[copied=true]:opacity-100 no-hover:opacity-100"
        />
        <span className="text-sm text-muted-foreground">{op.summary}</span>
        {access && (
          <span className="ml-auto text-xs text-muted-foreground" title={t(`api.accessHint.${access}`)} data-access={access}>
            {t(`api.access.${access}`)}
            {op["x-sentra-permission"] && (
              <>
                {" · "}
                <span data-permission={op["x-sentra-permission"]}>{t("api.permission", { permission: op["x-sentra-permission"] })}</span>
              </>
            )}
          </span>
        )}
      </div>
      {open && (
        <div id={panelId} className="space-y-4 border-t px-4 py-3">
          {op.description && <Markdown compact>{op.description}</Markdown>}
          <section>
            <h4 className="mb-1 text-sm font-semibold">{t("api.parameters")}</h4>
            <Parameters doc={doc} params={op.parameters ?? []} />
          </section>
          {body && (
            <section>
              <h4 className="mb-1 text-sm font-semibold">{t("api.requestBody")}</h4>
              <SchemaView doc={doc} schema={body} />
            </section>
          )}
          <section>
            <h4 className="mb-1 text-sm font-semibold">{t("api.responses")}</h4>
            <Responses doc={doc} responses={op.responses ?? {}} />
          </section>
        </div>
      )}
    </div>
  );
}

/** Read-only API reference rendered from the OpenAPI spec. Deliberately no "try it out": it never sends requests. */
export function ApiReference() {
  const { t } = useTranslation("help");
  const spec = useOpenApi();
  if (spec.isPending) return <Skeleton className="h-40 w-full" />;
  if (spec.isError) return <p role="alert">{t("loadError")}</p>;
  const doc = spec.data;
  return (
    <div className="max-w-4xl space-y-6" data-testid="api-reference">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">{t("api.title")}</h2>
        <p className="mt-1 text-muted-foreground">
          {t("api.intro", { title: doc.info?.title ?? "", version: doc.info?.version ?? "" })}
        </p>
        <p className="mt-1 text-sm text-muted-foreground">{t("api.readOnly")}</p>
      </div>
      {groupOperations(doc).map(([tag, ops]) => (
        <section key={tag} aria-labelledby={`api-tag-${tag}`} className="space-y-2">
          <h3 id={`api-tag-${tag}`} className="text-lg font-semibold">
            {tag}
          </h3>
          {doc.tags?.find((x) => x.name === tag)?.description && (
            <p className="text-sm text-muted-foreground">{doc.tags?.find((x) => x.name === tag)?.description}</p>
          )}
          {ops.map(({ method, path, op }) => (
            <Operation key={`${method} ${path}`} doc={doc} method={method} path={path} op={op} />
          ))}
        </section>
      ))}
    </div>
  );
}
