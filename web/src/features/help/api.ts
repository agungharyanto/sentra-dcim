import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { getJson } from "@/lib/api";

/** Mirrors api/openapi.yaml#/components/schemas/HelpMeta. */
export interface HelpMeta {
  slug: string;
  title: string;
  audience: "user" | "admin";
  menu?: string;
  order: number;
  headings: { level: number; text: string }[];
}

export interface HelpPage extends HelpMeta {
  locale: string;
  markdown: string;
}

export const HELP_HOME = "help";
export const API_REFERENCE_PATH = "/help/api/reference";

function useLocale() {
  const { i18n } = useTranslation();
  return i18n.resolvedLanguage ?? i18n.language;
}

/** Page index of the active language (re-fetched when the language changes). */
export function useHelpIndex() {
  const locale = useLocale();
  return useQuery({
    queryKey: ["help", "index", locale],
    queryFn: ({ signal }) =>
      getJson<{ locale: string; pages: HelpMeta[] }>(`/api/v1/help/index?locale=${encodeURIComponent(locale)}`, signal),
    staleTime: 5 * 60_000,
  });
}

export function useHelpPage(slug: string) {
  const locale = useLocale();
  return useQuery({
    queryKey: ["help", "page", locale, slug],
    // Slug segments are encoded individually; the server only matches them against its page whitelist.
    queryFn: ({ signal }) =>
      getJson<HelpPage>(
        `/api/v1/help/pages/${slug.split("/").map(encodeURIComponent).join("/")}?locale=${encodeURIComponent(locale)}`,
        signal,
      ),
    staleTime: 5 * 60_000,
  });
}

export function useOpenApi() {
  return useQuery({
    queryKey: ["help", "openapi"],
    queryFn: ({ signal }) => getJson<OpenApiDoc>("/api/v1/help/openapi", signal),
    staleTime: Infinity,
  });
}

/** Help page for a registry menu (front matter `menu`), else the Help Center home. */
export function helpPathForMenu(pages: HelpMeta[] | undefined, menuKey: string | undefined): string {
  const page = menuKey ? pages?.find((p) => p.menu === menuKey) : undefined;
  return page && page.slug !== HELP_HOME ? `/help/${page.slug}` : "/help";
}

// --- Minimal OpenAPI 3.1 typing (only what the read-only viewer renders) ---

export interface OpenApiSchema {
  $ref?: string;
  type?: string | string[];
  format?: string;
  description?: string;
  enum?: unknown[];
  example?: unknown;
  pattern?: string;
  items?: OpenApiSchema;
  properties?: Record<string, OpenApiSchema>;
  required?: string[];
  additionalProperties?: boolean | OpenApiSchema;
  allOf?: OpenApiSchema[];
  oneOf?: OpenApiSchema[];
  anyOf?: OpenApiSchema[];
}

export interface OpenApiParameter {
  $ref?: string;
  name: string;
  in: string;
  required?: boolean;
  description?: string;
  schema?: OpenApiSchema;
}

export interface OpenApiResponse {
  $ref?: string;
  description?: string;
  content?: Record<string, { schema?: OpenApiSchema }>;
}

export interface OpenApiOperation {
  tags?: string[];
  operationId?: string;
  summary?: string;
  description?: string;
  security?: Record<string, string[]>[];
  parameters?: OpenApiParameter[];
  requestBody?: { content?: Record<string, { schema?: OpenApiSchema }> };
  responses?: Record<string, OpenApiResponse>;
}

export interface OpenApiDoc {
  openapi: string;
  info?: { title?: string; version?: string };
  tags?: { name: string; description?: string }[];
  paths: Record<string, Record<string, OpenApiOperation>>;
  components?: {
    schemas?: Record<string, OpenApiSchema>;
    parameters?: Record<string, OpenApiParameter>;
    responses?: Record<string, OpenApiResponse>;
  };
}
