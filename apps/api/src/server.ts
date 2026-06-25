import { buildApp } from "./app.js";
import { config } from "./config.js";
import { connectMongo } from "./lib/mongo.js";

const app = buildApp();

async function start() {
  await connectMongo();
  app.log.info("MongoDB connected");
  const address = await app.listen({ port: config.PORT, host: "0.0.0.0" });
  app.log.info(`API listening at ${address}`);
}

start().catch((err) => {
  app.log.error(err);
  process.exit(1);
});
