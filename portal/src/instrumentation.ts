export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { registerOTel } = await import("@vercel/otel");
    registerOTel({
      serviceName: "portal",
      attributes: {
        "deployment.environment": process.env.ENV ?? "local",
      },
    });
  }
}
