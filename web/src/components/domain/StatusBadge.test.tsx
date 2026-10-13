import { render, screen } from "@testing-library/react";

import { MenuStatusBadge } from "@/components/domain/MenuStatusBadge";
import { dcimStatuses, menuStatuses } from "@/components/domain/status";
import { StatusBadge } from "@/components/domain/StatusBadge";
import { StatusDot } from "@/components/domain/StatusDot";
import { initI18n } from "@/i18n";
import { resources, supportedLocales } from "@/i18n/resources";

describe.each(supportedLocales)("status components (%s)", (lng) => {
  beforeEach(async () => {
    await initI18n(lng);
  });

  it.each(dcimStatuses)("StatusBadge %s shows an icon and the translated label", (status) => {
    const { container } = render(<StatusBadge status={status} />);
    const badge = container.querySelector(`[data-status="${status}"]`);
    expect(badge).toHaveTextContent(resources[lng].ds.status[status]);
    const icon = badge?.querySelector("svg");
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(badge?.className).toContain(`bg-status-${status}-bg`);
  });

  it.each(dcimStatuses)("StatusDot %s keeps the label for screen readers when compact", (status) => {
    const { container } = render(<StatusDot status={status} hideLabel />);
    const dot = container.querySelector(`[data-status="${status}"]`);
    expect(dot?.querySelector("svg")).not.toBeNull();
    expect(dot).toHaveAttribute("title", resources[lng].ds.status[status]);
    expect(screen.getByText(resources[lng].ds.status[status])).toHaveClass("sr-only");
  });

  it.each(menuStatuses)("MenuStatusBadge %s shows an icon and the translated label", (status) => {
    const { container } = render(<MenuStatusBadge status={status} />);
    const badge = container.querySelector(`[data-menu-status="${status}"]`);
    expect(badge).toHaveTextContent(resources[lng].shell.menuStatus[status]);
    expect(badge?.querySelector("svg")).not.toBeNull();
  });
});
