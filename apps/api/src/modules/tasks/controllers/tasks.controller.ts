import * as taskService from "../services";

// Route debug: quan sát các task agent tạo qua tool.
export const getTasks = async () => taskService.getTasks();
