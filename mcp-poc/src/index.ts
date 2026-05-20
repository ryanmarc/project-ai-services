#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { SSEServerTransport } from "@modelcontextprotocol/sdk/server/sse.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
} from "@modelcontextprotocol/sdk/types.js";
import { exec } from "child_process";
import { promisify } from "util";
import express, { Request, Response } from "express";

const execAsync = promisify(exec);

// Define the tool schema for application create
const CREATE_APPLICATION_TOOL: Tool = {
  name: "create_application",
  description: "Creates an AI services application using the ai-services CLI. Deploys an application with the provided name based on the specified template.",
  inputSchema: {
    type: "object",
    properties: {
      name: {
        type: "string",
        description: "Application name (required)",
      },
      template: {
        type: "string",
        description: "Application template to use (required). Use 'ai-services application templates' to view available templates.",
      },
      params: {
        type: "array",
        items: {
          type: "string",
        },
        description: "Inline parameters to configure the application. Format: key=value pairs. Example: ['key1=value1', 'key2=value2']. These override values from --values files.",
      },
      values: {
        type: "array",
        items: {
          type: "string",
        },
        description: "Specify values files to override default template values. Can provide multiple files; later files override earlier ones.",
      },
      skipValidation: {
        type: "array",
        items: {
          type: "string",
        },
        description: "Skip specific validation checks during application creation.",
      },
      skipImageDownload: {
        type: "boolean",
        description: "Skip container image pull/download during application creation. Use only if required images already exist locally. (Podman runtime only)",
      },
      skipModelDownload: {
        type: "boolean",
        description: "Skip model download during application creation. Use if local models already exist at /var/lib/ai-services/models/. (Podman runtime only)",
      },
      imagePullPolicy: {
        type: "string",
        enum: ["Always", "Never", "IfNotPresent"],
        description: "Image pull policy for container images. 'Always': pull every time, 'Never': use only local images, 'IfNotPresent': pull only if not present locally. Defaults to 'IfNotPresent'. (Podman runtime only)",
      },
      timeout: {
        type: "string",
        description: "Timeout for the operation (e.g., '10s', '2m', '1h'). (OpenShift runtime only)",
      },
    },
    required: ["name", "template"],
  },
};

// Create server instance
const server = new Server(
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

// Handle list tools request
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [CREATE_APPLICATION_TOOL],
  };
});

// Handle tool execution
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  if (request.params.name !== "create_application") {
    throw new Error(`Unknown tool: ${request.params.name}`);
  }

  const args = request.params.arguments as {
    name: string;
    template: string;
    params?: string[];
    values?: string[];
    skipValidation?: string[];
    skipImageDownload?: boolean;
    skipModelDownload?: boolean;
    imagePullPolicy?: string;
    timeout?: string;
  };

  // Validate required arguments
  if (!args.name) {
    throw new Error("Missing required argument: name");
  }
  if (!args.template) {
    throw new Error("Missing required argument: template");
  }

  // Build the command
  let command = `ai-services application create ${args.name} --template ${args.template}`;

  // Add optional flags
  if (args.params && args.params.length > 0) {
    command += ` --params ${args.params.join(",")}`;
  }

  if (args.values && args.values.length > 0) {
    for (const valueFile of args.values) {
      command += ` --values ${valueFile}`;
    }
  }

  if (args.skipValidation && args.skipValidation.length > 0) {
    command += ` --skip-validation ${args.skipValidation.join(",")}`;
  }

  if (args.skipImageDownload === true) {
    command += " --skip-image-download";
  }

  if (args.skipModelDownload === true) {
    command += " --skip-model-download";
  }

  if (args.imagePullPolicy) {
    command += ` --image-pull-policy ${args.imagePullPolicy}`;
  }

  if (args.timeout) {
    command += ` --timeout ${args.timeout}`;
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
});

// Start the server with SSE transport over HTTP
async function main() {
  const app = express();
  const PORT = parseInt(process.env.PORT || "3000", 10);
  const HOST = process.env.HOST || "0.0.0.0";

  app.get("/health", (_req: Request, res: Response) => {
    res.json({ status: "ok", server: "mcp-poc" });
  });

  app.get("/sse", async (req: Request, res: Response) => {
    console.error("New SSE connection established");

    const transport = new SSEServerTransport("/message", res);
    await server.connect(transport);

    req.on("close", () => {
      console.error("SSE connection closed");
    });
  });

  app.post("/message", async (req: Request, res: Response) => {
    // This endpoint is used by the SSE transport for client messages
    res.status(200).end();
  });

  app.listen(PORT, HOST as string, () => {
    console.error(`MCP POC Server running on http://${HOST}:${PORT}`);
    console.error(`SSE endpoint: http://${HOST}:${PORT}/sse`);
    console.error(`Health check: http://${HOST}:${PORT}/health`);
  });
}

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});