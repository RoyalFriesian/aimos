package builtins

import "github.com/Sarnga/agent-platform/pkg/skills"

// RegisterAll registers all built-in skill handlers with the given registry.
func RegisterAll(reg *skills.Registry) {
	// Messaging
	reg.Register(postMessageSkill)

	// Supervision (CEO/Manager)
	reg.Register(checkChildSkill)
	reg.Register(createWorkerSkill)
	reg.Register(resolveConflictSkill)
	reg.Register(mergeBranchSkill)

	// Todos
	reg.Register(createTodoSkill)
	reg.Register(completeTodoSkill)
	reg.Register(blockTodoSkill)
	reg.Register(startTodoSkill)

	// Lifecycle
	reg.Register(markDoneSkill)
	reg.Register(deliverWorkSkill)
	reg.Register(updateSummarySkill)
	reg.Register(escalateSkill)
	reg.Register(scheduleFollowupSkill)

	// Files
	reg.Register(writeFileSkill)
	reg.Register(readFileSkill)
	reg.Register(runQASkill)

	// Control
	reg.Register(noOpSkill)
}
