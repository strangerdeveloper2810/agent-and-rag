import { z } from "zod";
import { zodToJsonSchema } from "zod-to-json-schema";
import {
  createTaskInputSchema,
  updateTaskInputSchema,
  listTasksInputSchema,
} from "../schemas/task";
import {
  createTask,
  updateTask,
  listTasks,
  deleteTask,
} from "../modules/tasks/tasks.repository";
import { embed } from "../lib/voyage";
import {
  searchSimilar,
  listSources,
} from "../modules/documents/documents.repository";

type Tool = {
  name: string;
  description: string;
  schema: z.ZodTypeAny;
  execute: (input: any) => Promise<unknown>;
};

const ragSearchSchema = z.object({
  query: z.string().describe("Câu truy vấn để tìm trong tài liệu"),
});

const listDocumentsSchema = z.object({});

const deleteTaskSchema = z.object({ id: z.string() });

const tools: Tool[] = [
  {
    name: "ragSearch",
    description:
      "Tìm kiếm thông tin trong các tài liệu người dùng đã nạp. Dùng khi câu hỏi liên quan đến nội dung tài liệu.",
    schema: ragSearchSchema,
    execute: async ({ query }: z.infer<typeof ragSearchSchema>) => {
      const [vec] = await embed([query], "query");
      const results = await searchSimilar(vec, 5);
      return results;
    },
  },
  {
    name: "listDocuments",
    description:
      "Liệt kê các tài liệu đã được nạp (tên file + số chunk). Dùng khi người dùng hỏi 'có bao nhiêu tài liệu' hoặc 'có những tài liệu nào'.",
    schema: listDocumentsSchema,
    execute: async () => listSources(),
  },
  {
    name: "createTask",
    description:
      "Tạo một task/công việc mới. Trích xuất title, priority, tags, dueDate, remindAt từ yêu cầu người dùng.",
    schema: createTaskInputSchema,
    execute: async (input) =>
      createTask(createTaskInputSchema.parse(input), "agent"),
  },
  {
    name: "listTasks",
    description: "Liệt kê task, có thể lọc theo status, priority, hoặc tag.",
    schema: listTasksInputSchema,
    execute: async (input) => listTasks(listTasksInputSchema.parse(input)),
  },
  {
    name: "updateTask",
    description: "Cập nhật một task theo id (đổi status, priority, title...).",
    schema: updateTaskInputSchema,
    execute: async (input) => updateTask(updateTaskInputSchema.parse(input)),
  },
  {
    name: "deleteTask",
    description: "Xóa một task theo id.",
    schema: deleteTaskSchema,
    execute: async ({ id }: z.infer<typeof deleteTaskSchema>) => deleteTask(id),
  },
];

// Format gửi cho Anthropic API
export const toolDefinitions = tools.map((t) => ({
  name: t.name,
  description: t.description,
  input_schema: zodToJsonSchema(t.schema, { target: "openApi3" }) as Record<
    string,
    unknown
  >,
}));

export function getTool(name: string): Tool | undefined {
  return tools.find((t) => t.name === name);
}
