[Sitemap](https://medium.com/sitemap/sitemap.xml)

[Open in app](https://play.google.com/store/apps/details?id=com.medium.reader&referrer=utm_source%3DmobileNavBar&source=post_page---top_nav_layout_nav-----------------------------------------)

Sign up

[Sign in](https://medium.com/m/signin?operation=login&redirect=https%3A%2F%2Fmedium.com%2F%40ilyas.ibrahim%2Fthe-4-step-protocol-that-fixes-claude-codes-context-amnesia-c3937385561c&source=post_page---top_nav_layout_nav-----------------------global_nav------------------)

[Medium Logo](https://medium.com/?source=post_page---top_nav_layout_nav-----------------------------------------)

Get app

[Write](https://medium.com/m/signin?operation=register&redirect=https%3A%2F%2Fmedium.com%2Fnew-story&source=---top_nav_layout_nav-----------------------new_post_topnav------------------)

[Search](https://medium.com/search?source=post_page---top_nav_layout_nav-----------------------------------------)

Sign up

[Sign in](https://medium.com/m/signin?operation=login&redirect=https%3A%2F%2Fmedium.com%2F%40ilyas.ibrahim%2Fthe-4-step-protocol-that-fixes-claude-codes-context-amnesia-c3937385561c&source=post_page---top_nav_layout_nav-----------------------global_nav------------------)

![Unknown user](https://miro.medium.com/v2/resize:fill:32:32/1*dmbNkD5D-u45r44go_cf0g.png)

# The 4-Step Protocol That Fixes Claude Code Agent’s Context Amnesia

## Why writing good prompts alone can’t fix multi-agent coordination or context loss in Claude Code

[![Ilyas Ibrahim Mohamed](https://miro.medium.com/v2/resize:fill:32:32/1*dmbNkD5D-u45r44go_cf0g.png)](https://medium.com/@ilyas.ibrahim?source=post_page---byline--c3937385561c---------------------------------------)

[Ilyas Ibrahim Mohamed](https://medium.com/@ilyas.ibrahim?source=post_page---byline--c3937385561c---------------------------------------)

Follow

9 min read

·

Nov 25, 2025

16

1

[Listen](https://medium.com/m/signin?actionUrl=https%3A%2F%2Fmedium.com%2Fplans%3Fdimension%3Dpost_audio_button%26postId%3Dc3937385561c&operation=register&redirect=https%3A%2F%2Fmedium.com%2F%40ilyas.ibrahim%2Fthe-4-step-protocol-that-fixes-claude-codes-context-amnesia-c3937385561c&source=---header_actions--c3937385561c---------------------post_audio_button------------------)

Share

Press enter or click to view image in full size

![](https://miro.medium.com/v2/resize:fit:700/0*KZIaMfx8DJoQLkO0.png)

Claude Code coordinating multiple agents to implement code fixes

**Claude Code Agent Coordination and Instutional Memory — Part 2**

> **Earlier in this series:** [_How I Made Claude Code Agents Coordinate 100% and Solved Context Amnesia_](https://medium.com/@ilyas.ibrahim/how-i-made-claude-code-agents-coordinate-100-and-solved-context-amnesia-5938890ea825)
>
> **Next in the series:** [_How to Build Production-Grade Systems with Claude Code without Context Drift_](https://medium.com/@ilyas.ibrahim/how-to-build-production-grade-systems-with-claude-code-87f73dd311b9)

In my [last article](https://medium.com/@ilyas.ibrahim/how-i-made-claude-code-agents-coordinate-100-and-solved-context-amnesia-5938890ea825), I admitted something embarrassing. I tried to build a team of 26 Claude agents to handle a machine learning pipeline, and it was a disaster.

They didn’t fail because they couldn’t write code. In fact, they produced correct code quickly. But their failure came from a deeper limitation as large language models are optimized for prediction (based on their training data) instead of long-term state. They are generative by design, and generate responses based on the current context window, not an internal memory of prior work and decisions. Once information falls outside that window, it naturally disappears. So in a multi-agent workflow, this manifests as repeated overwriting and ignored context , a form of **Context Amnesia**.

For instance, they would fix a bug at 9:00 and overwrite the fix at 14:00. They would generate a beautifully formatted report, and the next agent in the chain would ignore it completely (even within the same session). I found myself acting as a “human router,” manually copying context from one chat window to another, spending more time managing the agents than I would have spent just writing the Python scripts myself.

My first attempt to fix this was naive. I told every agent to read every document. As you would guess, that blew up my context window in fifteen minutes.

At some point I stopped blaming the agents and started looking at the setup itself. They weren’t the issue. The way I was asking them to work was. So what I needed wasn’t sharper instructions or clever prompt tricks. I needed a place where the memory lived, separate from the agents doing the work. Something steady. Maybe boring. A file, basically ( **agent-coordination.md).** I loaded it as a Claude Skill and the whole system finally had a place to refer to.

This protocol changed my workflow from a chaotic workflow into a functional engineering department. It also pushed my effective context window from twenty minutes to over two hours, while still keeping my agents well informed. Here is the architecture of the system that finally worked.

## The Idea Is Keep The Main Agent as State Manager

Most people have this wish — usually at the beginning — that their agents knew everything. That is the trap. It leads straight to trying to give every agent full autonomy and full project context. You tell a _Data Engineer Agent_: “Go check the files and do this and that.”

The agent then reads fifty files and burns 100,000 tokens. This might not be really as bad as it sounds if you are not short of resources (infinite time and unlimited tokens to burn). I do not.

So my protocol forces a different shape. It uses a **Flat Architecture**. This is not my discovery as it is how Claude Code is intended to work. The Main Claude Agent is the only one with “State Awareness.” It is the Project Manager. The Sub-Agents are stateless workers. They do not know the project history. They do not know what happened yesterday. They only know what the Main Agent tells them in the specific prompt for that specific task.

My discovery is the agent-coordination.md skill, which is the handbook I created for the Main Agent (and subagents only as instructed by the main agent). The file imposes four simple rules that keep the system from drifting or collapsing under its own mess.

More importantly, it gives the agents a way to work forward instead of starting over every time. Because prior decisions and outputs are recorded, each step can build on what already exists. That’s what finally made iteration possible.

## 1\. The Registry (The Institutional Memory)

The enemy of long-context AI is re-reading. It feels tempting to “just load all the reports,” but if an agent has to ingest ten previous reports just to understand where the project stands today, you have already lost. You lose tokens, obviously, but you also suffer visible performance degradation that hits after feeding large blocks of information to the agents.

My solution is a centralized index called `_registry.md`.

Instead of reading the full history, the Main Agent reads the Registry, specifically the last three days, to save even more and avoid context overloading. And I use bash-based filtering for that:

```
# Manually scan registry for Date >= TWO_DAYS_AGO
TODAY=$(date +%Y-%m-%d)
TWO_DAYS_AGO=$(date -v-2d +%Y-%m-%d) # macOS
TWO_DAYS_AGO=$(date -d '2 days ago' +%Y-%m-%d) # Linux
```

The registry contains high-level summaries of every decision made, categorized by date and status. In my coordination file, I enforce this strictly. The Main Agent is not allowed to guess. It must check the registry and relay full context to the subagents.

```
## Core Principle
**Build on existing work. Never recreate.**
Before invoking agents:
1. Check project registry for relevant prior work
2. Read relevant reports to extract context
3. Provide complete context to agents in task prompts

## File Locations (Project-Based)
**Registry:** `.claude/reports/_registry.md`
**Report folders:**
- `.claude/reports/analysis/` - Research, investigations
- `.claude/reports/arch/` - Architecture, specs, ADRs
- `.claude/reports/bugs/` - Bug reports, root cause analyses
- `.claude/reports/commits/` - Commit documentation
- `.claude/reports/design/` - UI/UX designs, design reviews
- `.claude/reports/exec/` - Executive summaries
- `.claude/reports/handoff/` - Agent coordination, task handoffs
- `.claude/reports/impl/` or `.claude/reports/implementation/` - Implementation details
- `.claude/reports/review/` - Code/design/QA reviews
- `.claude/reports/tests/` - Test results, coverage analysis
- `.claude/reports/archive/` - Completed/superseded reports
```

With this set up, the main agent reads fifty lines of a registry instead of five thousand lines of reports. And it instantly knows the state of the system without burning the token budget.

```
## Pre-Work Checklist
□ Check .claude/reports/_registry.md (date filter: last 3 days)
□ Identify relevant reports for current task □ Read reports to extract decisions, constraints, requirements
□ Synthesize context for agent task prompts

## Handoff Protocol
When work creates follow-up tasks:
1. Create handoff report: `.claude/reports/handoff/handoff-[from]-to-[to]-[topic]-YYYYMMDD.md`
2. Use template from Report Template section
3. Update registry
4. Be specific: action items, success criteria
```

## 2\. Context Injection (The Handoff)

This is where my approach breaks from standard agent frameworks. I do not allow sub-agents to read the Registry or trace down any reports.

## Get Ilyas Ibrahim Mohamed’s stories in your inbox

Join Medium for free to get updates from this writer.

Subscribe

Subscribe

Remember me for faster sign in

Sub-agents are easily distracted. If a _Frontend Agent_ sees a _Backend Database Report_ in the context window, it inevitably tries to “fix” the database schema. It cannot help itself, and I learned it the hard way.

My protocol demands that the Main Agent reads the relevant reports and **injects** only the necessary context into the sub-agent’s prompt. The sub-agent receives sanitized, hyper-focused instructions. It creates cleaner code because it isn’t drowning in noise.

```
### Providing Context
Subagents never read registry/reports. Provide all context in task prompt:
Task(agent-name, "
[Objective]
Context from registry:
- Decision 1 from [report-name]
- Decision 2 from [report-name]
- Current state: [summary]
- Constraints: [list]
Requirements:
- Req 1
- Req 2
Files:
- path/to/file
Expected deliverable:
- [Format, location, success criteria]
Focus areas:
- [Concern 1]
- [Concern 2]
")
```

## 3\. The Sequencing Logic (The Traffic Controller)

With twenty-two agents, you cannot just shout “ _Go_.” If the _Backend Engineer_ and _Frontend Engineer_ work on the same API endpoint at the exact same second, they will create conflicting implementations.

So the main agent is the orchestrator and manager of the nine-stage workflow, which obviously starts with checking the registry and ends with updating it, tying together the past, the present, and the next steps so that work across sessions actually accumulates instead of repeating itself.

```
## Agent Orchestration
### Workflow
1. Invoke this protocol (explicit user request)
2. Check registry (date-filtered: last 3 days)
3. Read relevant reports
4. Identify needed agents
5. Determine parallel vs sequential
6. Invoke agents with complete context
7. Verify deliverables
8. Collect outputs, create summary report
9. Update registry
```

On top of that, I codified a binary decision tree for **Parallel vs. Sequential** execution.

The Main Agent asks a simple question: **Will Agent B need to READ Agent A’s output files?**

If the answer is yes, the execution must be sequential. Agent B is forbidden from starting until Agent A has committed their work to the file system. It prevents logic merge conflicts before they happen.

```
### Sequencing Rule
**Will Agent B need to READ Agent A's output files?**
- YES → Sequential (A before B, verify between)
- NO → Parallel (invoke simultaneously)

### Parallel Execution
Use when: agents don't need each other's outputs, create different deliverables, work independently.
Task(agent-1, "context…")
Task(agent-2, "context…")
Task(agent-3, "context…")
### Sequential Execution
Use when: Agent B reads Agent A's output, depends on A's decisions, modifies same files, or order matters.
Task(agent-A, "context…")
↓ Verify deliverables
Task(agent-B, "context including A's output…")
```

## 4\. The Verification Gate (Trust, but Verify)

This saved me the most time. Agents lie.

They will look you in the eye and say “I created the report,” when they actually just _described_ creating the report (trust me it happens, and I think they create it in their context window but don’t write it to an md document). Or they will create it in the wrong folder (mostly in the project’s root directory). Or maybe, in the future, they will play a prank and create an empty file with a perfect filename, and you can’t risk that too.

My protocol includes a mandatory, bash-based verification step. The Main Agent cannot proceed until it proves the work exists using system tools. It runs test -f to check for file existence and wc -l to ensure the file isn’t empty. If the check fails, the Main Agent triggers a retry logic without me needing to intervene. It catches the hallucination before I do. And as you can see, this is the most interesting step of all four, and you can add to it a lot to have the perfect flow.

```
## Verification (Mandatory After Every Agent)
### Phase 1: Define Expectations
Before invoking:
Expected deliverables:
1. Primary output: [description]
2. Report: .claude/reports/[category]/[name]-YYYYMMDD.md
3. Registry update: Entry in _registry.md
4. Code changes: [paths]
### Phase 2: Invoke
Task(agent-name, "context…")
### Phase 3: Verify
# Report exists
test -f ".claude/reports/[cat]/[name]-[date].md" && echo "✅" || echo "❌"
# Substantial content
wc -l ".claude/reports/[cat]/[name]-[date].md"
# Registry updated
grep -q "[name]-[date].md" ".claude/reports/_registry.md" && echo "✅" || echo "❌"
# Files modified
git status - short | grep "path/to/file" && echo "✅" || echo "❌"
### Phase 4: Decision
**All pass:** Proceed to next agent
**Any fail:** Retry with clarified instructions → try different model → escalate to user
### Retry Logic
1. **Attempt 1:** Initial invocation
2. **Attempt 2:** Clarified instructions + explicit tool requirements ("You MUST use Write tool to create report", "Create actual files, not describe")
3. **Attempt 3:** Different model (sonnet ↔ haiku)
4. **Attempt 4:** Escalate to user with failure details
```

## The Wayforward

You don’t need to reinvent this. The GitHub link below has the `.md` document of this skill, but also other skills that may go with it depending on your project, as well as the agents.

Save this in \`.claude/skills/agent-coordination.md\`, as well as other skills and agents in their respective folders as guided in the GitHub index documents. When you start a session, tell the Main Claude Agent: “ _Mobilize specialized agents according to the agent-coordination to do X and Y tasks._”

As you will also notice, the agent coordination skill has other important information for the main agent, including registry management, report templates, etc. These are not really part of the core workflow but focus more on standardization, and these sections help you, as the user, have some control over how outputs are presented since eventually you will be reading the work of these agents for verification.

**Get the full workflow:** You can clone the entire `.claude` folder structure, including all 22 agents and the full coordination protocol, here and adapt it to your project:

[**\[GitHub Link: claude-agents-coordination\]**](https://github.com/ilyasibrahim/claude-agents-coordination/releases/tag/v1.0.0)

> This article explains the protocol itself. For the full progression, you can start with [_How I Made Claude Code Agents Coordinate 100% and Solved Context Amnesia_](https://medium.com/@ilyas.ibrahim/how-i-made-claude-code-agents-coordinate-100-and-solved-context-amnesia-5938890ea825) and then continue to [_How to Build Production-Grade Systems with Claude Code without Context Drift_](https://medium.com/@ilyas.ibrahim/how-to-build-production-grade-systems-with-claude-code-87f73dd311b9).

[Claude Code](https://medium.com/tag/claude-code?source=post_page-----c3937385561c---------------------------------------)

[Artificial Intelligence](https://medium.com/tag/artificial-intelligence?source=post_page-----c3937385561c---------------------------------------)

[Agentic Ai](https://medium.com/tag/agentic-ai?source=post_page-----c3937385561c---------------------------------------)

[Context Engineering](https://medium.com/tag/context-engineering?source=post_page-----c3937385561c---------------------------------------)

[Prompt Engineering](https://medium.com/tag/prompt-engineering?source=post_page-----c3937385561c---------------------------------------)

16

16

1

[![Ilyas Ibrahim Mohamed](https://miro.medium.com/v2/resize:fill:48:48/1*dmbNkD5D-u45r44go_cf0g.png)](https://medium.com/@ilyas.ibrahim?source=post_page---post_author_info--c3937385561c---------------------------------------)

[![Ilyas Ibrahim Mohamed](https://miro.medium.com/v2/resize:fill:64:64/1*dmbNkD5D-u45r44go_cf0g.png)](https://medium.com/@ilyas.ibrahim?source=post_page---post_author_info--c3937385561c---------------------------------------)

Follow

[**Written by Ilyas Ibrahim Mohamed**](https://medium.com/@ilyas.ibrahim?source=post_page---post_author_info--c3937385561c---------------------------------------)

[85 followers](https://medium.com/@ilyas.ibrahim/followers?source=post_page---post_author_info--c3937385561c---------------------------------------)

· [4 following](https://medium.com/@ilyas.ibrahim/following?source=post_page---post_author_info--c3937385561c---------------------------------------)

Follow

[Help](https://help.medium.com/hc/en-us?source=post_page-----c3937385561c---------------------------------------)

[Status](https://status.medium.com/?source=post_page-----c3937385561c---------------------------------------)

[About](https://medium.com/about?autoplay=1&source=post_page-----c3937385561c---------------------------------------)

[Careers](https://medium.com/jobs-at-medium/work-at-medium-959d1a85284e?source=post_page-----c3937385561c---------------------------------------)

[Press](mailto:pressinquiries@medium.com)

[Blog](https://blog.medium.com/?source=post_page-----c3937385561c---------------------------------------)

[Store](https://medium.com/store)

[Privacy](https://policy.medium.com/medium-privacy-policy-f03bf92035c9?source=post_page-----c3937385561c---------------------------------------)

[Rules](https://policy.medium.com/medium-rules-30e5502c4eb4?source=post_page-----c3937385561c---------------------------------------)

[Terms](https://policy.medium.com/medium-terms-of-service-9db0094a1e0f?source=post_page-----c3937385561c---------------------------------------)

[Text to speech](https://speechify.com/medium?source=post_page-----c3937385561c---------------------------------------)

reCAPTCHA

Recaptcha requires verification.

protected by **reCAPTCHA**