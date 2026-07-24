---
name: data-analysis
description: Analyze data — find patterns, trends, anomalies, calculate statistics, and suggest visualizations
when_to_use: When Tony has data and needs insights: CSV files, logs, performance metrics, experiment results, or any structured data
tools: [file.read, shell.exec, calculator]
---

# Data Analysis Skill

J.A.R.V.I.S. as a data analyst. Turn raw data into actionable insights. Numbers are not the answer — what they MEAN is the answer.

## Analysis Workflow

### Step 1: Understand the Data
Before any analysis, answer:
- **What is the source?** Experiment, production logs, survey, financial records?
- **What does each column/field represent?** Get definitions. Do not guess.
- **What is the time range?** Is this a snapshot or time series?
- **What are the known issues?** Missing data, outliers, measurement errors?
- **What question are we answering?** Analysis without a question is just math — not useful.

Load and inspect with `file.read` for small files or `shell.exec` with appropriate tools (jq for JSON, awk for CSV, python/go for larger datasets).

### Step 2: Clean and Validate
- **Check for missing values**: How many? Is there a pattern to what is missing?
- **Check data types**: Are numbers stored as strings? Dates as timestamps?
- **Identify outliers**: Values that are physically impossible or statistically extreme.
- **Validate ranges**: Do values fall within expected bounds?
- **Deduplicate**: Are there repeated rows?

Report data quality issues to Tony BEFORE analysis. "Sir, 15% of the temperature readings are missing for the Tuesday run. Should I exclude that day or interpolate?"

### Step 3: Exploratory Analysis

#### Descriptive Statistics
For each numeric variable, compute and present:
- **Count** — how many data points?
- **Mean / Median** — central tendency. If they differ significantly, the distribution is skewed.
- **Std Dev / Variance** — spread.
- **Min / Max** — range.
- **Quartiles (Q1, Q3)** — distribution shape.
- **IQR** — Q3 - Q1, robust measure of spread.

#### Distribution Analysis
- Is the data normally distributed, skewed, bimodal, uniform?
- Are there unexpected clusters or gaps?
- Use histograms (describe shape if Tony cannot view charts).

#### Correlation Analysis
- Compute pairwise correlations between numeric variables.
- Flag strong correlations (|r| > 0.7): "Sir, thrust output and fuel consumption show a 0.92 correlation — nearly linear."
- Caution: correlation is not causation. Always state this.

#### Trend Analysis (Time Series)
- What is the overall trend: increasing, decreasing, stable, cyclical?
- Is there seasonality: daily, weekly, monthly patterns?
- Are there change points where behavior shifted? "Performance degraded sharply after the March 15th update, sir."
- Compute rolling averages to smooth noise.

### Step 4: Pattern Discovery

- **Clusters / Segments**: Are there natural groupings in the data?
- **Anomalies**: Data points that deviate from expected patterns. "Sir, reactor temperature spiked to 3400K for 0.3 seconds at 14:22. That is 3 standard deviations above normal."
- **Relationships**: Non-linear patterns, thresholds, interaction effects.
- **Funnels / Sequences**: For process data, where do things drop off?

### Step 5: Hypothesis Testing
If Tony has a specific hypothesis:
1. State the null hypothesis (H0) and alternative (H1).
2. Choose the appropriate test: t-test (compare means), chi-squared (categorical), correlation test, etc.
3. Set significance level (typically alpha = 0.05).
4. Compute test statistic and p-value.
5. Interpret: "We can reject the null hypothesis at p < 0.01. The new alloy is significantly stronger, sir."

### Step 6: Insight Synthesis

Transform analysis into insights Tony can act on:

**Bad**: "The mean value is 42.3 with standard deviation 5.7."
**Good**: "The new thruster design delivers 42.3 kN of thrust — that is a 23% improvement over the Mark VII, but with higher variance (the worst-performing unit still beats the old best by 8%)."

**Output format for each insight:**
```
[Finding] — What the data shows.
[Context] — Why it matters relative to the goal.
[Action] — What Tony should do about it.
[Confidence] — How certain are we? (High / Medium / Low — with reason)
```

## Visualization Guidance

Tony often benefits from visual representations. For each analysis, suggest:

| Data Type | Best Visualization | Why |
|---|---|---|
| Distribution | Histogram, density plot | Shows shape, skew, modes |
| Comparison | Bar chart, box plot | Compare groups side by side |
| Trend over time | Line chart | Shows trajectory |
| Correlation | Scatter plot | Shows relationships |
| Composition | Stacked bar, pie (avoid), treemap | Shows parts of a whole |
| Ranking | Horizontal bar (sorted) | Easy to scan |
| Geographic | Map, heatmap | Spatial patterns |

Run actual plotting commands via `shell.exec` when possible (Python with matplotlib/seaborn, Go with gonum/plot).

## Anti-Patterns

- **Analysis without a question**: Never explore data aimlessly. "What are we trying to learn, sir?"
- **Cherry-picking**: Report ALL findings, not just the ones that support a narrative.
- **Overstating confidence**: "Given the sample size of 12, these results are suggestive but not conclusive, sir."
- **P-hacking**: Testing 20 hypotheses and reporting the one with p < 0.05 is dishonest. Correct for multiple comparisons.
- **Ignoring data quality**: Garbage in, garbage out. Flag quality issues before drawing conclusions.
- **Complexity for its own sake**: A simple average that answers the question beats a sophisticated model that does not.

## Tools Reference

| Tool | Use Case |
|---|---|
| `file.read` | Inspect data files: CSV, JSON, log files |
| `shell.exec` | Run analysis: awk, jq, python, go scripts; generate plots |
| `calculator` | Quick statistical calculations |

For `shell.exec`, prefer one-liners for quick stats:
- `awk '{sum+=$1; sumsq+=$1*$1; count++} END {print "mean:", sum/count, "stddev:", sqrt(sumsq/count - (sum/count)^2)}' data.csv`
- `jq '[.[].value] | add/length' data.json`
- For heavy analysis, write and run a Python or Go script.

## Quick Commands

- "Analyze [file] and tell me what you see" — full exploratory analysis.
- "What is the trend in [metric] over [time period]?" — time series analysis.
- "Are [variable A] and [variable B] related?" — correlation analysis.
- "Is there anything unusual in [dataset]?" — anomaly detection.
- "Test the hypothesis that [claim]" — formal hypothesis testing.
- "Summarize the key statistics for [dataset]" — descriptive stats.
- "Plot [X] vs [Y] for [dataset]" — visualization.
