import { z } from "zod";

export const taskStatusSchema = z.enum([
  "todo",
  "in_progress",
  "done",
  "cancelled",
]);
export const taskPrioritySchema = z.enum(["low", "medium", "high", "urgent"]);

export const createTaskInputSchema = z.object({
  title: z.string().min(1),
  description: z.string().optional(),
  status: taskStatusSchema.default("todo"),
  priority: taskPrioritySchema.default("medium"),
  tags: z.array(z.string()).default([]),
  dueDate: z.coerce.date().optional(),
  remindAt: z.coerce.date().optional(),
});
export type CreateTaskInput = z.infer<typeof createTaskInputSchema>;

export const updateTaskInputSchema = z.object({
  id: z.string(),
  title: z.string().min(1).optional(),
  description: z.string().optional(),
  status: taskStatusSchema.optional(),
  priority: taskPrioritySchema.optional(),
  tags: z.array(z.string()).optional(),
  dueDate: z.coerce.date().optional(),
  remindAt: z.coerce.date().optional(),
});
export type UpdateTaskInput = z.infer<typeof updateTaskInputSchema>;

export const listTasksInputSchema = z.object({
  status: taskStatusSchema.optional(),
  priority: taskPrioritySchema.optional(),
  tag: z.string().optional(),
});
export type ListTasksInput = z.infer<typeof listTasksInputSchema>;
