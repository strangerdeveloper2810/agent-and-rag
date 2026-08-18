import { z } from "zod";

export const updateSettingsSchema = z.object({
  persona_preset: z
    .enum(["default", "coder", "business", "creative", "custom"])
    .optional(),
  formality: z.enum(["casual", "neutral", "formal"]).optional(),
  verbosity: z.enum(["concise", "normal", "detailed"]).optional(),
  humor: z.enum(["none", "dry", "playful"]).optional(),
  custom_instructions: z.string().max(2000, "Tối đa 2000 ký tự").optional(),
});

export type UpdateSettingsInput = z.infer<typeof updateSettingsSchema>;
