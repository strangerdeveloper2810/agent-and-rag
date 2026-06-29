import { listTasks } from "../repositories";

// Business logic tầng task (cho route debug). Tool agent dùng repository trực tiếp.
export const getTasks = () => listTasks({});
