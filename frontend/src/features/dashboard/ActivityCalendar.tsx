import { useState } from "react";
import { cx } from "../../components/ui/cx";
import type { ActivityDay } from "../../lib/api/types";
import { LEVELS, calendarWeeks, dayTotal, describeDay, formatDay, level, monthLabels, type Level } from "./activity";

const levelClasses: Record<Level, string> = {
  0: "bg-paper-sunken",
  1: "bg-activity-1",
  2: "bg-activity-2",
  3: "bg-activity-3",
  4: "bg-activity-4",
};

const WEEKDAYS = ["Mon", "", "Wed", "", "Fri", "", ""];

type Hovered = {
  day: ActivityDay;
  week: number;
  left: number;
  top: number;
};

/**
 * What the student did on each day of the last twelve weeks, as a calendar:
 * one column per week, Monday at the top, darker for more done. The cells
 * are a picture only; the summary line and the table below carry the same
 * numbers for keyboards and screen readers, and pointing at a day shows its
 * numbers too.
 */
export function ActivityCalendar({ activity, today }: { activity: readonly ActivityDay[]; today: string }) {
  const [hovered, setHovered] = useState<Hovered | null>(null);
  const weeks = calendarWeeks(activity, today);
  const months = monthLabels(weeks);
  const shown = weeks.flatMap((w) => w.days.filter((d) => d !== null));
  const active = shown.filter((d) => dayTotal(d) > 0);

  return (
    <div>
      <p className="text-sm text-ink-muted">
        {active.length === 0
          ? "Nothing done in the last 12 weeks yet."
          : `Active on ${String(active.length)} ${active.length === 1 ? "day" : "days"} in the last 12 weeks.`}
      </p>

      <div
        className="relative mt-4 inline-flex"
        onPointerLeave={() => {
          setHovered(null);
        }}
      >
        <div aria-hidden="true" className="mr-1.5 flex flex-col text-[10px] leading-[18px] text-ink-subtle">
          <span className="h-[18px]" />
          {WEEKDAYS.map((name, i) => (
            <span key={i} className="h-[18px]">
              {name}
            </span>
          ))}
        </div>
        <div aria-hidden="true" data-testid="activity-calendar" className="flex">
          {weeks.map((week, w) => (
            <div key={week.start} className="flex w-[18px] flex-col">
              <span className="h-[18px] overflow-visible text-[10px] leading-[18px] whitespace-nowrap text-ink-subtle">
                {months[w]}
              </span>
              {week.days.map((day, d) =>
                day ? (
                  // The hit box is the whole 18px step, the 2px gap included.
                  <div
                    key={day.date}
                    data-date={day.date}
                    className="size-[18px] p-px"
                    onPointerEnter={(event) => {
                      setHovered({
                        day,
                        week: w,
                        left: event.currentTarget.offsetLeft + 9,
                        top: event.currentTarget.offsetTop,
                      });
                    }}
                  >
                    <div
                      className={cx(
                        "size-full rounded-[3px]",
                        levelClasses[level(dayTotal(day))],
                        hovered?.day.date === day.date && "outline-2 outline-ink",
                      )}
                    />
                  </div>
                ) : (
                  <div key={d} className="size-[18px]" />
                ),
              )}
            </div>
          ))}
        </div>

        {hovered && (
          <div
            role="tooltip"
            className={cx(
              "pointer-events-none absolute z-10 w-max max-w-56 -translate-y-full rounded-sm border border-rule bg-paper-raised px-3 py-2 shadow-sm",
              // Kept inside the chart at its edges.
              hovered.week < 3 ? "-translate-x-4" : hovered.week > 8 ? "-translate-x-[calc(100%-1rem)]" : "-translate-x-1/2",
            )}
            style={{ left: hovered.left, top: hovered.top - 6 }}
          >
            <p className="text-sm font-medium text-ink">{describeDay(hovered.day)}</p>
            <p className="text-xs text-ink-muted">{formatDay(hovered.day.date)}</p>
          </div>
        )}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-muted">
        <span>Things done a day:</span>
        <LegendItem level={0} label="none" />
        {LEVELS.map((step) => (
          <LegendItem key={step.level} level={step.level} label={step.label} />
        ))}
      </div>

      <details className="mt-4 text-sm">
        <summary className="cursor-pointer text-ink-muted hover:text-ink">Show the days as a table</summary>
        {active.length === 0 ? (
          <p className="mt-3 text-ink-muted">No days with activity yet.</p>
        ) : (
          <table className="mt-3 w-full text-left tabular-nums">
            <caption className="sr-only">Days with activity in the last 12 weeks, newest first</caption>
            <thead className="text-xs text-ink-muted">
              <tr className="border-b border-rule">
                <th scope="col" className="py-1.5 font-medium">
                  Day
                </th>
                <th scope="col" className="py-1.5 text-right font-medium">
                  Words
                </th>
                <th scope="col" className="py-1.5 text-right font-medium">
                  Highlights
                </th>
                <th scope="col" className="py-1.5 text-right font-medium">
                  Pages
                </th>
                <th scope="col" className="py-1.5 text-right font-medium">
                  Cards
                </th>
              </tr>
            </thead>
            <tbody>
              {active.toReversed().map((day) => (
                <tr key={day.date} className="border-b border-rule text-ink">
                  <th scope="row" className="py-1.5 font-normal">
                    {formatDay(day.date)}
                  </th>
                  <td className="py-1.5 text-right">{day.words}</td>
                  <td className="py-1.5 text-right">{day.highlights}</td>
                  <td className="py-1.5 text-right">{day.pages}</td>
                  <td className="py-1.5 text-right">{day.reviews}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </details>
    </div>
  );
}

function LegendItem({ level: step, label }: { level: Level; label: string }) {
  return (
    <span className="inline-flex items-center gap-1">
      <span aria-hidden="true" className={cx("size-3 rounded-[3px]", levelClasses[step])} />
      {label}
    </span>
  );
}
