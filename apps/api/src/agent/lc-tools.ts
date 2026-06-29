import { tool } from "@langchain/core/tools";
import { z } from "zod";
import {
  createTaskInputSchema,
  updateTaskInputSchema,
  listTasksInputSchema,
} from "../schemas/task";
import {
  createTask,
  listTasks,
  updateTask,
  deleteTask,
} from "../modules/tasks/tasks.repository";
import { embed } from "../lib/voyage";
import {
  searchSimilar,
  listDocuments,
  getDocumentContent,
} from "../modules/documents/documents.repository";

const ragSearch = tool(
  async ({ query }) => {
    const [vec] = await embed([query], "query");
    const results = await searchSimilar(vec, 5);
    return JSON.stringify(results);
  },
  {
    name: "ragSearch",
    description:
      "Tìm kiếm thông tin trong các tài liệu người dùng đã nạp. Dùng khi câu hỏi liên quan đến nội dung tài liệu.",
    schema: z.object({
      query: z.string().describe("Câu truy vấn để tìm trong tài liệu"),
    }),
  },
);

const listDocumentsTool = tool(
  async () => JSON.stringify(await listDocuments()),
  {
    name: "listDocuments",
    description:
      "Liệt kê các tài liệu đã được nạp (tên file + số chunk). Dùng khi người dùng hỏi 'có bao nhiêu tài liệu' hoặc 'có những tài liệu nào'.",
    schema: z.object({}),
  },
);

const readDocumentTool = tool(
  async ({ source }) => JSON.stringify(await getDocumentContent(source)),
  {
    name: "readDocument",
    description:
      "Đọc TOÀN BỘ nội dung của MỘT tài liệu theo tên file. Dùng khi người dùng muốn xem nội dung đầy đủ của một tài liệu cụ thể. Nếu chưa biết tên file, gọi listDocuments trước.",
    schema: z.object({
      source: z
        .string()
        .describe("Tên file tài liệu cần đọc, ví dụ test-rag.txt"),
    }),
  },
);

const createTaskTool = tool(
  async (input) =>
    JSON.stringify(
      await createTask(createTaskInputSchema.parse(input), "agent"),
    ),
  {
    name: "createTask",
    description:
      "Tạo một task/công việc mới. Trích xuất title, priority, tags, dueDate, remindAt từ yêu cầu người dùng.",
    schema: createTaskInputSchema,
  },
);

const listTasksTool = tool(
  async (input) =>
    JSON.stringify(await listTasks(listTasksInputSchema.parse(input))),
  {
    name: "listTasks",
    description: "Liệt kê task, có thể lọc theo status, priority, hoặc tag.",
    schema: listTasksInputSchema,
  },
);

const updateTaskTool = tool(
  async (input) =>
    JSON.stringify(await updateTask(updateTaskInputSchema.parse(input))),
  {
    name: "updateTask",
    description: "Cập nhật một task theo id (đổi status, priority, title...).",
    schema: updateTaskInputSchema,
  },
);

const deleteTaskTool = tool(
  async ({ id }) => JSON.stringify(await deleteTask(id)),
  {
    name: "deleteTask",
    description: "Xóa một task theo id.",
    schema: z.object({ id: z.string() }),
  },
);

export const lcTools = [
  ragSearch,
  listDocumentsTool,
  readDocumentTool,
  createTaskTool,
  listTasksTool,
  updateTaskTool,
  deleteTaskTool,
];
