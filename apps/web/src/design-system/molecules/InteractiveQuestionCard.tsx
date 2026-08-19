import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import type { ClarifyQuestion, ClarifyOption } from "@/types";
import {
  CheckIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  PaperAirplaneIcon,
  SparklesIcon,
  QuestionMarkCircleIcon,
} from "@heroicons/react/24/outline";

export interface InteractiveQuestionCardProps {
  questions: ClarifyQuestion[];
  disabled?: boolean;
  onSubmit: (answerText: string) => void;
}

// Kiểm tra nội dung text có chứa ký tự tiếng Việt có dấu hay không
const isVietnameseText = (text: string): boolean => {
  if (!text) return false;
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    if (
      (code >= 0x00c0 && code <= 0x00ff) ||
      (code >= 0x0100 && code <= 0x024f) ||
      (code >= 0x1ea0 && code <= 0x1ef9)
    ) {
      return true;
    }
  }
  return false;
};

export const InteractiveQuestionCard: React.FC<
  InteractiveQuestionCardProps
> = ({ questions, disabled = false, onSubmit }) => {
  const { i18n } = useTranslation();
  const isEnLocale = i18n?.language?.startsWith("en") || false;

  const [currentStep, setCurrentStep] = useState(0);
  const [answers, setAnswers] = useState<Record<number, string>>({});
  const [selectedMulti, setSelectedMulti] = useState<Record<number, number[]>>(
    {},
  );
  const [customText, setCustomText] = useState("");

  if (!questions || questions.length === 0) return null;

  // Tự động nhận diện ngôn ngữ của câu hỏi: nếu không có tiếng Việt hoặc locale là en -> dùng English UI
  const hasVietnamese = questions.some(
    (q) =>
      isVietnameseText(q.prompt || "") ||
      isVietnameseText(q.question || "") ||
      isVietnameseText(q.header || "") ||
      (q.options || []).some(
        (opt) =>
          isVietnameseText(opt.label) ||
          isVietnameseText(opt.description || ""),
      ),
  );
  const isEn = isEnLocale || !hasVietnamese;

  const currentQ = questions[currentStep] || questions[0];
  const isMulti = currentQ.multiSelect === true;
  const isLastStep = currentStep === questions.length - 1;
  const options = currentQ.options || [];

  // Trích xuất nội dung câu hỏi rõ ràng (fallback prompt -> question -> header -> mặc định)
  const questionPrompt =
    currentQ.prompt ||
    currentQ.question ||
    currentQ.header ||
    (isEn
      ? "Please select the appropriate option to proceed:"
      : "Vui lòng chọn phương án phù hợp để tiếp tục:");

  const buildSummary = (finalAnswers: Record<number, string>) => {
    if (questions.length === 1) {
      const qText =
        questions[0].prompt ||
        questions[0].question ||
        questions[0].header ||
        (isEn ? "Option" : "Lựa chọn");
      const aText = finalAnswers[0] || "";
      return `Q: ${qText}\nA: ${aText}`;
    }

    return questions
      .map((q, idx) => {
        const qText =
          q.prompt ||
          q.question ||
          q.header ||
          (isEn ? `Question ${idx + 1}` : `Câu hỏi ${idx + 1}`);
        const aText = finalAnswers[idx] || "";
        return `Q: ${qText}\nA: ${aText}`;
      })
      .join("\n\n");
  };

  const handleSingleSelect = (label: string) => {
    if (disabled) return;
    const nextAnswers = { ...answers, [currentStep]: label };
    setAnswers(nextAnswers);

    if (isLastStep) {
      onSubmit(buildSummary(nextAnswers));
    } else {
      setCurrentStep((prev) => prev + 1);
      setCustomText("");
    }
  };

  const toggleMultiOption = (optIdx: number) => {
    if (disabled) return;
    const current = selectedMulti[currentStep] || [];
    const next = current.includes(optIdx)
      ? current.filter((i) => i !== optIdx)
      : [...current, optIdx];
    setSelectedMulti({ ...selectedMulti, [currentStep]: next });
  };

  const handleMultiSubmit = () => {
    if (disabled) return;
    const selectedIdxs = selectedMulti[currentStep] || [];
    const selectedLabels = selectedIdxs
      .map((i) => options[i]?.label)
      .filter(Boolean);
    if (customText.trim()) {
      selectedLabels.push(customText.trim());
    }

    if (selectedLabels.length === 0) return;

    const answerStr = selectedLabels.join(", ");
    const nextAnswers = { ...answers, [currentStep]: answerStr };
    setAnswers(nextAnswers);

    if (isLastStep) {
      onSubmit(buildSummary(nextAnswers));
    } else {
      setCurrentStep((prev) => prev + 1);
      setCustomText("");
    }
  };

  const handleCustomTextSubmit = () => {
    if (disabled || !customText.trim()) return;
    handleSingleSelect(customText.trim());
  };

  return (
    <div className="my-4 rounded-2xl border border-primary/30 bg-card/95 backdrop-blur-md p-4 sm:p-5 shadow-xl transition-all">
      {/* Top Header with Category & Step counter */}
      <div className="flex items-center justify-between border-b border-border/60 pb-3 mb-3.5">
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <SparklesIcon className="h-4 w-4" />
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold uppercase tracking-wider text-primary">
              {currentQ.header ||
                (isEn ? "Clarify & Planning" : "Làm rõ yêu cầu & Kế hoạch")}
            </span>
            {isMulti ? (
              <span className="rounded-full bg-indigo-500/15 border border-indigo-500/30 px-2 py-0.5 text-[10px] font-bold text-indigo-500">
                {isEn ? "Multi-select" : "Chọn nhiều"}
              </span>
            ) : (
              <span className="rounded-full bg-sky-500/15 border border-sky-500/30 px-2 py-0.5 text-[10px] font-bold text-sky-500">
                {isEn ? "Single Choice" : "Chọn 1"}
              </span>
            )}
          </div>
        </div>

        {questions.length > 1 && (
          <span className="text-xs font-semibold text-muted-foreground bg-muted/80 px-2.5 py-1 rounded-full border border-border/50">
            {isEn
              ? `Question ${currentStep + 1} / ${questions.length}`
              : `Câu ${currentStep + 1} / ${questions.length}`}
          </span>
        )}
      </div>

      {/* Prominent Question Prompt Box */}
      <div className="mb-4 rounded-xl bg-primary/5 border border-primary/20 p-3.5 flex items-start gap-2.5">
        <div className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-primary/20 text-primary">
          <QuestionMarkCircleIcon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <h4 className="text-sm font-semibold text-foreground leading-snug">
            {questionPrompt}
          </h4>
          <p className="mt-1 text-[11px] font-medium text-muted-foreground">
            {isMulti
              ? isEn
                ? "✦ Multiple choices allowed — select one or more and click Next."
                : "✦ Có thể chọn nhiều phương án — tích chọn và bấm Tiếp tục."
              : isEn
                ? "✦ Single choice — click an option to select and proceed instantly."
                : "✦ Lựa chọn duy nhất — bấm 1 phương án để tiếp tục ngay."}
          </p>
        </div>
      </div>

      {/* Options List */}
      <div className="space-y-2">
        {options.map((opt: ClarifyOption, idx: number) => {
          const isChecked = (selectedMulti[currentStep] || []).includes(idx);
          return (
            <button
              key={`${idx}-${opt.label}`}
              type="button"
              disabled={disabled}
              onClick={() =>
                isMulti ? toggleMultiOption(idx) : handleSingleSelect(opt.label)
              }
              className={`w-full group text-left rounded-xl border p-3 sm:p-3.5 transition-all duration-200 flex items-start justify-between gap-3 ${
                isChecked
                  ? "border-primary bg-primary/15 shadow-sm ring-1 ring-primary/40"
                  : "border-border/80 bg-background/50 hover:border-primary/60 hover:bg-accent/40"
              } ${disabled ? "opacity-60 cursor-not-allowed" : "cursor-pointer"}`}
            >
              <div className="flex items-start gap-3 min-w-0">
                <div
                  className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center text-xs font-bold transition-colors ${
                    isMulti
                      ? `rounded-md border ${
                          isChecked
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-border bg-muted text-muted-foreground group-hover:border-primary/50 group-hover:bg-primary/10"
                        }`
                      : `rounded-full border ${
                          isChecked
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-border/80 bg-muted/60 text-muted-foreground group-hover:border-primary group-hover:bg-primary/20 group-hover:text-primary"
                        }`
                  }`}
                >
                  {isMulti ? (
                    isChecked ? (
                      <CheckIcon className="h-3.5 w-3.5 stroke-[3]" />
                    ) : (
                      idx + 1
                    )
                  ) : (
                    idx + 1
                  )}
                </div>

                <div className="min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-xs font-bold text-foreground group-hover:text-primary transition-colors">
                      {opt.label}
                    </span>
                    {opt.recommended && (
                      <span className="rounded-md bg-emerald-500/15 px-1.5 py-0.5 text-[10px] font-bold text-emerald-600 dark:text-emerald-400">
                        {isEn ? "Recommended" : "Khuyến nghị"}
                      </span>
                    )}
                  </div>
                  {opt.description && (
                    <p className="text-[11px] text-muted-foreground mt-0.5 leading-relaxed">
                      {opt.description}
                    </p>
                  )}
                </div>
              </div>

              {!isMulti && (
                <ArrowRightIcon className="h-4 w-4 shrink-0 text-muted-foreground opacity-40 group-hover:opacity-100 group-hover:translate-x-0.5 transition-all text-primary" />
              )}
            </button>
          );
        })}
      </div>

      {/* Free-text Write-in Input */}
      <div className="mt-3 pt-3 border-t border-border/50 flex items-center gap-2">
        <input
          type="text"
          value={customText}
          disabled={disabled}
          onChange={(e) => setCustomText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              if (isMulti) {
                handleMultiSubmit();
              } else {
                handleCustomTextSubmit();
              }
            }
          }}
          placeholder={
            isEn
              ? "Or type your custom answer / opinion..."
              : "Hoặc nhập phương án / ý kiến riêng của bạn..."
          }
          className="flex-1 rounded-xl border border-border/70 bg-background/80 px-3 py-2 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
        />

        {!isMulti && (
          <button
            type="button"
            disabled={disabled || !customText.trim()}
            onClick={handleCustomTextSubmit}
            className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-xs font-semibold text-primary-foreground transition-all hover:bg-primary/90 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <span>{isEn ? "Send" : "Gửi"}</span>
            <PaperAirplaneIcon className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* Wizard Navigation Footer (Multi-select or Multi-step) */}
      {(isMulti || questions.length > 1) && (
        <div className="mt-3 pt-2.5 flex items-center justify-between">
          {currentStep > 0 ? (
            <button
              type="button"
              disabled={disabled}
              onClick={() => setCurrentStep((s) => Math.max(0, s - 1))}
              className="flex items-center gap-1 text-xs font-semibold text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            >
              <ArrowLeftIcon className="h-3.5 w-3.5" />
              {isEn ? "Back" : "Quay lại"}
            </button>
          ) : (
            <div />
          )}

          {isMulti && (
            <button
              type="button"
              disabled={
                disabled ||
                ((selectedMulti[currentStep] || []).length === 0 &&
                  !customText.trim())
              }
              onClick={handleMultiSubmit}
              className="flex items-center gap-1.5 rounded-xl bg-primary px-4 py-2 text-xs font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-all disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            >
              <span>
                {isLastStep
                  ? isEn
                    ? "Submit Selection"
                    : "Hoàn tất lựa chọn"
                  : isEn
                    ? `Next (${(selectedMulti[currentStep] || []).length}) →`
                    : `Tiếp tục (${(selectedMulti[currentStep] || []).length}) →`}
              </span>
            </button>
          )}
        </div>
      )}
    </div>
  );
};

export default InteractiveQuestionCard;
