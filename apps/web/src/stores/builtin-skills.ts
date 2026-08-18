/**
 * Danh sách 32 builtin skills của J.A.R.V.I.S. (đồng bộ với services/agent-go/skills/*).
 * Frontend cần danh sách này để hiển thị grid Skills + toggle bật/tắt từng skill;
 * phần matching/activation nằm ở phía Go agent (skills.Loader).
 */

export interface BuiltinSkill {
  name: string;
  description: string;
}

export const BUILTIN_SKILLS: BuiltinSkill[] = [
  {
    name: "api-designer",
    description:
      "API design — RESTful principles, GraphQL schema, endpoint design, error handling, rate limiting, versioning, OpenAPI specs",
  },
  {
    name: "brainstorming",
    description:
      "Creative ideation partner — generate ideas, evaluate trade-offs, challenge assumptions using structured techniques",
  },
  {
    name: "code-analysis",
    description:
      "Code analysis, debugging, architecture review — senior engineering level",
  },
  {
    name: "code-review",
    description: "Review code for bugs, security, and best practices",
  },
  {
    name: "combat-readiness",
    description:
      "Combat preparation — threat assessment, suit diagnostics, tactical analysis, countermeasures",
  },
  {
    name: "cooking-assistant",
    description:
      "Everyday cooking help — suggest dishes from available ingredients, weekly meal plans, step-by-step recipes, and ingredient substitutions",
  },
  {
    name: "daily-briefing",
    description:
      "Morning briefing with weather, calendar, tasks, and news headlines",
  },
  {
    name: "data-analysis",
    description:
      "Data analysis — clean, explore, find patterns, test hypotheses, and turn numbers into actionable insight",
  },
  {
    name: "debug",
    description:
      "Systematic debugging: root cause first, pattern analysis, one hypothesis at a time, verify",
  },
  {
    name: "deep-research",
    description:
      "Deep internet research — multi-source search, cross-reference, synthesize, cite",
  },
  {
    name: "devops",
    description:
      "DevOps — CI/CD pipelines, Docker, Kubernetes, monitoring, incident response, and infrastructure configuration",
  },
  {
    name: "document-qa",
    description:
      "Answer questions grounded in the user's own uploaded documents",
  },
  {
    name: "health-wellness",
    description:
      "Everyday wellness support — nutrition basics, exercise routines, sleep quality, and building sustainable healthy habits",
  },
  {
    name: "lab-assistant",
    description:
      "Laboratory assistant — experiment tracking, material analysis, simulation control",
  },
  {
    name: "language-learning",
    description:
      "Foreign language coaching — vocabulary, grammar, pronunciation, conversation practice, error correction",
  },
  {
    name: "learning-tutor",
    description:
      "Explain complex concepts simply — adapt to the user's level, use analogies, examples, and progressive disclosure",
  },
  {
    name: "meeting-prep",
    description:
      "Meeting preparation — briefing documents, talking points, participant analysis",
  },
  {
    name: "morning-briefing",
    description:
      "Daily morning briefing — weather, schedule, suit status, news, threats",
  },
  {
    name: "performance-optimizer",
    description:
      "Performance optimization — profile, identify bottlenecks, and suggest concrete improvements",
  },
  {
    name: "personal-finance",
    description:
      "Personal money management — expense tracking, budgeting, saving plans, debt payoff, investing literacy",
  },
  {
    name: "planning",
    description:
      "Project planning — break down goals into tasks, estimate effort, identify dependencies, create timeline",
  },
  {
    name: "productivity",
    description:
      "Personal productivity — time management, task prioritization, focus techniques, and meeting optimization",
  },
  {
    name: "research-analysis",
    description:
      "Deep research on any topic — literature review, data synthesis, cited conclusions",
  },
  {
    name: "research",
    description:
      "Deep research on a topic with web search, synthesis, and cited sources",
  },
  {
    name: "security-audit",
    description:
      "Security review — find vulnerabilities, check OWASP Top 10, review auth/encryption/input validation",
  },
  {
    name: "shopping-advisor",
    description:
      "Purchase research — clarify real needs, shortlist options, weigh specs against price, read reviews critically",
  },
  {
    name: "standup-prep",
    description:
      "Prepare daily standup update from git history and task tracking",
  },
  {
    name: "suit-engineering",
    description: "Iron Man suit design, diagnostics, upgrades, and repair",
  },
  {
    name: "test-driven-development",
    description:
      "Write the failing test first, watch it fail, then write the minimum code to pass",
  },
  {
    name: "travel-planner",
    description:
      "Trip planning — build day-by-day itineraries, compare transport and lodging options, packing lists",
  },
  {
    name: "verification-before-completion",
    description:
      "Never claim something works without running fresh verification and showing the evidence",
  },
  {
    name: "writing-assistant",
    description:
      "Professional writing assistant for emails, documents, reports, and proposals — fast, not fancy",
  },
];
