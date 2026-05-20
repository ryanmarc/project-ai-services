#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";
import { exec } from "child_process";
import { promisify } from "util";
import express, { Request, Response } from "express";

const execAsync = promisify(exec);

// Create server instance
const server = new McpServer(
  {
    name: "mcp-poc",
    version: "1.0.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Define the input schema using Zod
const CreateApplicationSchema = z.object({
  name: z.string().describe("Application name (required)"),
  template: z.string().describe("Application template to use (required). Use 'ai-services application templates' to view available templates."),
  params: z.array(z.string()).optional().describe("Inline parameters to configure the application. Format: key=value pairs. Example: ['key1=value1', 'key2=value2']. These override values from --values files."),
  values: z.array(z.string()).optional().describe("Specify values files to override default template values. Can provide multiple files; later files override earlier ones."),
  skipValidation: z.array(z.string()).optional().describe("Skip specific validation checks during application creation."),
  skipImageDownload: z.boolean().optional().describe("Skip container image pull/download during application creation. Use only if required images already exist locally. (Podman runtime only)"),
  skipModelDownload: z.boolean().optional().describe("Skip model download during application creation. Use if local models already exist at /var/lib/ai-services/models/. (Podman runtime only)"),
  imagePullPolicy: z.enum(["Always", "Never", "IfNotPresent"]).optional().describe("Image pull policy for container images. 'Always': pull every time, 'Never': use only local images, 'IfNotPresent': pull only if not present locally. Defaults to 'IfNotPresent'. (Podman runtime only)"),
  timeout: z.string().optional().describe("Timeout for the operation (e.g., '10s', '2m', '1h'). (OpenShift runtime only)"),
});

// Register the tool using the modern API
server.registerTool(
  "create_application",
  {
    description: "Creates an AI services application using the ai-services CLI. Deploys an application with the provided name based on the specified template.",
    inputSchema: CreateApplicationSchema,
  },
  async (args) => {
    const typedArgs = args as z.infer<typeof CreateApplicationSchema>;

    // Validate required arguments
    if (!typedArgs.name) {
      throw new Error("Missing required argument: name");
    }
    if (!typedArgs.template) {
      throw new Error("Missing required argument: template");
    }

    // Build the command
    let command = `ai-services application create ${typedArgs.name} --template ${typedArgs.template} --runtime podman`;

    // Add optional flags
    if (typedArgs.params && typedArgs.params.length > 0) {
      command += ` --params ${typedArgs.params.join(",")}`;
    }

    if (typedArgs.values && typedArgs.values.length > 0) {
      for (const valueFile of typedArgs.values) {
        command += ` --values ${valueFile}`;
      }
    }

    if (typedArgs.skipValidation && typedArgs.skipValidation.length > 0) {
      command += ` --skip-validation ${typedArgs.skipValidation.join(",")}`;
    }

    if (typedArgs.skipImageDownload === true) {
      command += " --skip-image-download";
    }

    if (typedArgs.skipModelDownload === true) {
      command += " --skip-model-download";
    }

    if (typedArgs.imagePullPolicy) {
      command += ` --image-pull-policy ${typedArgs.imagePullPolicy}`;
    }

    if (typedArgs.timeout) {
      command += ` --timeout ${typedArgs.timeout}`;
    }

    try {
      // Execute the command
      const { stdout, stderr } = await execAsync(command);

      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(
              {
                success: true,
                command: command,
                stdout: stdout,
                stderr: stderr,
              },
              null,
              2
            ),
          },
        ],
      };
    } catch (error: any) {
      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(
              {
                success: false,
                command: command,
                error: error.message,
                stdout: error.stdout || "",
                stderr: error.stderr || "",
              },
              null,
              2
            ),
          },
        ],
        isError: true,
      };
    }
  }
);

// Start the server with StreamableHTTP transport over HTTP
async function main() {
  const app = express();
  const PORT = parseInt(process.env.PORT || "3000", 10);
  const HOST = process.env.HOST || "0.0.0.0";

  // Parse JSON bodies before handling requests
  app.use(express.json());

  app.get("/health", (_req: Request, res: Response) => {
    res.json({ status: "ok", server: "mcp-poc" });
  });

  // Handle MCP requests using the StreamableHTTP transport
  app.post("/mcp", async (req: Request, res: Response) => {
    console.error("New MCP request received");
    console.error("Request body:", JSON.stringify(req.body, null, 2));

    try {
      // Create a new transport instance for each request
      const transport = new StreamableHTTPServerTransport();

      // Close any existing connection before connecting a new one
      try {
        await server.server.close();
      } catch (closeError) {
        // Ignore errors if not connected
        console.error("Note: No existing connection to close");
      }

      // Connect the server to the new transport
      await server.server.connect(transport);

      // Handle the request
      await transport.handleRequest(req, res, req.body);

      // Close the connection after handling the request
      await server.server.close();
    } catch (error: any) {
      console.error("Error handling MCP request:", error);
      if (!res.headersSent) {
        res.status(500).json({
          error: "Internal server error",
          message: error.message,
          stack: error.stack
        });
      }
    }
  });

  app.listen(PORT, HOST as string, () => {
    console.error(`MCP POC Server running on http://${HOST}:${PORT}`);
    console.error(`MCP endpoint: http://${HOST}:${PORT}/mcp`);
    console.error(`Health check: http://${HOST}:${PORT}/health`);
  });
}

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});