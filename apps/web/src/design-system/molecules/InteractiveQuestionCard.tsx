import React, { useState } from "react";
import type { ClarifyQuestion, ClarifyOption } from "@/types";
import {
  CheckIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  PaperAirplaneIcon,
  SparklesIcon,
} from "@heroicons/react/24/outline";

export interface InteractiveQuestionCardProps {
  questions: ClarifyQuestion[];
  disabled?: boolean;
  onSubmit: (answerText: string) => void;
}

export const InteractiveQuestionCard: React.FC<InteractiveQuestionCardProps> = ({
  questions,
  disabled = false,
  onSubmit,
}) => {
  const [currentStep, setCurrentStep] = useState(0);
  const [answers, setAnswers] = useState<Record<number, string>>({});
  const [selectedMulti, setSelectedMulti] = useState<Record<number, number[]>>({});
  const [customText, setCustomText] = useState("");

  if (!questions || questions.length === 0) return null;

  const currentQ = questions[currentStep] || questions[0];
  const isMulti = currentQ.multiSelect === true;
  const isLastStep = currentStep === questions.length - 1;
  const options = currentQ.options || [];

  const handleSingleSelect = (label: string) => {
    if (disabled) return;
    if (questions.length === 1) {
      onSubmit(label);
      return;
    }

    const nextAnswers = { ...answers, [currentStep]: label };
    setAnswers(nextAnswers);

    if (isLastStep) {
      const summary = questions
        .map((q, idx) => `${q.header || q.prompt}: ${nextAnswers[idx] || label}`)
        .join("\n");
      onSubmit(summary);
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
    const selectedLabels = selectedIdxs.map((i) => options[i]?.label).filter(Boolean);
    if (customText.trim()) {
      selectedLabels.push(customText.trim());
    }

    if (selectedLabels.length === 0) return;

    const answerStr = selectedLabels.join(", ");
    if (questions.length === 1) {
      onSubmit(answerStr);
      return;
    }

    const nextAnswers = { ...answers, [currentStep]: answerStr };
    setAnswers(nextAnswers);

    if (isLastStep) {
      const summary = questions
        .map((q, idx) => `${q.header || q.prompt}: ${nextAnswers[idx] || answerStr}`)
        .join("\n");
      onSubmit(summary);
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
    <div className="my-4 rounded-2xl border border-primary/20 bg-card/90 backdrop-blur-md p-4 shadow-lg transition-all">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border/60 pb-3 mb-3.5">
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <SparklesIcon className="h-4 w-4" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold text-primary uppercase tracking-wider">
                {currentQ.header || "Làm rõ kế hoạch"}
              </span>
              {isMulti && (
                <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                  Chọn nhiều
                </span>
              )}
            </div>
            <h4 className="text-sm font-medium text-foreground mt-0.5">
              {currentQ.prompt}
            </h4>
          </div>
        </div>

        {questions.length > 1 && (
          <span className="text-xs font-medium text-muted-foreground bg-muted/60 px-2.5 py-1 rounded-full">
            Bước {currentStep + 1} / {questions.length}
          </span>
        )}
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
              onClick={() => (isMulti ? toggleMultiOption(idx) : handleSingleSelect(opt.label))}
              className={`w-full group text-left rounded-xl border p-3 transition-all duration-200 flex items-start justify-between gap-3 ${
                isChecked
                  ? "border-primary bg-primary/10 shadow-sm"
                  : "border-border/70 hover:border-primary/50 hover:bg-accent/40"
              } ${disabled ? "opacity-60 cursor-not-allowed" : "cursor-pointer"}`}
            >
              <div className="flex items-start gap-2.5 min-w-0">
                <div
                  className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md text-xs font-semibold transition-colors ${
                    isChecked
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted text-muted-foreground group-hover:bg-primary/20 group-hover:text-primary"
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
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-semibold text-foreground group-hover:text-primary transition-colors">
                      {opt.label}
                    </span>
                    {opt.recommended && (
                      <span className="rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400">
                        Khuyến nghị
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
                <ArrowRightIcon className="h-4 w-4 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 group-hover:translate-x-0.5 transition-all text-primary" />
              )}
            </button>
          );
        })}
      </div>

      {/* Free-text Write-in Escape Hatch */}
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
          placeholder="Hoặc nhập phương án / yêu cầu riêng của bạn..."
          className="flex-1 rounded-xl border border-border/70 bg-background/80 px-3 py-2 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
        />

        {!isMulti && (
          <button
            type="button"
            disabled={disabled || !customText.trim()}
            onClick={handleCustomTextSubmit}
            className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-xs font-medium text-primary-foreground transition-all hover:bg-primary/90 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <span>Gửi</span>
            <PaperAirplaneIcon className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* Wizard Footer (Cho Multi-select hoặc Multi-step) */}
      {(isMulti || questions.length > 1) && (
        <div className="mt-3 pt-2.5 flex items-center justify-between">
          {currentStep > 0 ? (
            <button
              type="button"
              disabled={disabled}
              onClick={() => setCurrentStep((s) => Math.max(0, s - 1))}
              className="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeftIcon className="h-3.5 w-3.5" />
              Quay lại
            </button>
          ) : (
            <div />
          )}

          {isMulti && (
            <button
              type="button"
              disabled={
                disabled ||
                ((selectedMulti[currentStep] || []).length === 0 && !customText.trim())
              }
              onClick={handleMultiSubmit}
              className="flex items-center gap-1.5 rounded-xl bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <span>
                {isLastStep
                  ? "Hoàn tất lựa chọn"
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
