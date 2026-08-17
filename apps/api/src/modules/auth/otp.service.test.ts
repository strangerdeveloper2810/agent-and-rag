import { describe, it, expect } from "vitest";
import { generateOtp, hashOtp } from "./otp.service";

describe("generateOtp", () => {
  it("sinh mã 6 chữ số", () => {
    for (let i = 0; i < 20; i++) {
      const otp = generateOtp();
      expect(otp).toMatch(/^\d{6}$/);
    }
  });
});

describe("hashOtp", () => {
  it("cùng input cho cùng hash, khác input cho hash khác", () => {
    expect(hashOtp("123456")).toBe(hashOtp("123456"));
    expect(hashOtp("123456")).not.toBe(hashOtp("654321"));
  });

  it("không trả về plaintext OTP trong hash", () => {
    expect(hashOtp("123456")).not.toContain("123456");
  });
});
