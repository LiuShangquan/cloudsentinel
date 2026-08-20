import { describe, expect, it } from "vitest";
import { formatDuration, formatInterval, statusLabel, statusTone } from "./format";

describe("format helpers", () => {
  it("formats bounded probe durations", () => {
    expect(formatDuration(86)).toBe("86 ms");
    expect(formatDuration(1250)).toBe("1.25 s");
    expect(formatDuration(null)).toBe("—");
  });

  it("formats scheduler intervals", () => {
    expect(formatInterval(30)).toBe("30 秒");
    expect(formatInterval(120)).toBe("2 分钟");
    expect(formatInterval(7200)).toBe("2 小时");
  });

  it("maps domain states to consistent labels and tones", () => {
    expect(statusLabel("acknowledged")).toBe("已确认");
    expect(statusTone("failed")).toBe("danger");
    expect(statusTone("succeeded")).toBe("success");
  });
});
