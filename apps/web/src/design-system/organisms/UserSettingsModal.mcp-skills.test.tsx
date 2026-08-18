import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import i18n from "@/i18n";
import type { McpServer, UserSkill } from "@/stores/user.store";
import { UserSettingsModal } from "./UserSettingsModal";

// Mock cac store/hook phu thuoc de test tap trung vao phan UI moi gan cho
// avatar upload / MCP servers / skills - khong can goi API that.
vi.mock("@/stores/auth.store", () => ({
  useAuthStore: vi.fn(),
}));
vi.mock("@/stores/user.store", () => ({
  useUserStore: vi.fn(),
}));
vi.mock("@/design-system/molecules/Toast", () => ({
  useToast: vi.fn(),
}));
vi.mock("@/lib/upload", () => ({
  uploadImage: vi.fn(),
}));

import { useAuthStore } from "@/stores/auth.store";
import { useUserStore } from "@/stores/user.store";
import { useToast } from "@/design-system/molecules/Toast";
import { uploadImage } from "@/lib/upload";

const mockUseAuthStore = useAuthStore as unknown as Mock;
const mockUseUserStore = useUserStore as unknown as Mock;
const mockUseToast = useToast as unknown as Mock;
const mockUploadImage = uploadImage as unknown as Mock;

const toastApi = {
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
};

const mcpServer: McpServer = {
  id: "mcp-1",
  name: "Notion",
  transport: "sse",
  url: "https://notion.example.com/sse",
  api_key: null,
  enabled: true,
};

const customSkill: UserSkill = {
  id: "skill-1",
  name: "weekly-report",
  description: "Tong hop bao cao tuan",
  when_to_use: "Khi can bao cao",
  content: "Huong dan chi tiet...",
  triggers: ["report"],
  enabled: true,
};

let userStoreState: Record<string, unknown>;

const buildUserStoreState = () => ({
  isSaving: false,
  mcpServers: [] as McpServer[],
  skills: [] as UserSkill[],
  disabledBuiltinSkills: [] as string[],
  isLoadingMcp: false,
  isLoadingSkills: false,
  fetchSettings: vi.fn().mockResolvedValue({
    persona_preset: "default",
    formality: "neutral",
    verbosity: "normal",
    custom_instructions: "",
    agent_avatar_url: null,
  }),
  updateSettings: vi.fn().mockResolvedValue(undefined),
  updateProfile: vi.fn().mockResolvedValue({
    id: "u1",
    email: "trinh@x.com",
    name: "Trinh",
    role: "user",
  }),
  changePassword: vi.fn().mockResolvedValue(undefined),
  fetchMcpServers: vi.fn().mockResolvedValue([]),
  createMcpServer: vi.fn().mockResolvedValue(undefined),
  updateMcpServer: vi.fn().mockResolvedValue(undefined),
  deleteMcpServer: vi.fn().mockResolvedValue(undefined),
  fetchSkills: vi
    .fn()
    .mockResolvedValue({ customSkills: [], disabledBuiltinSkills: [] }),
  createSkill: vi.fn().mockResolvedValue(undefined),
  updateSkill: vi.fn().mockResolvedValue(undefined),
  deleteSkill: vi.fn().mockResolvedValue(undefined),
  toggleBuiltinSkill: vi.fn().mockResolvedValue(undefined),
});

const renderModal = (initialTab: "profile" | "persona" | "mcp" | "skills") =>
  render(
    <I18nextProvider i18n={i18n}>
      <UserSettingsModal isOpen onClose={vi.fn()} initialTab={initialTab} />
    </I18nextProvider>,
  );

describe("UserSettingsModal - avatar upload, MCP servers, skills", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    userStoreState = buildUserStoreState();
    mockUseUserStore.mockImplementation(() => userStoreState);
    mockUseAuthStore.mockReturnValue({
      user: {
        id: "u1",
        email: "trinh@x.com",
        name: "Trinh Nguyen",
        role: "user",
      },
    });
    mockUseToast.mockReturnValue(toastApi);
    mockUploadImage.mockResolvedValue("https://cdn.example.com/avatar.png");
    await i18n.changeLanguage("vi");
  });

  // -- Profile avatar upload --

  it("shows fallback initials when the user has no avatar yet", () => {
    renderModal("profile");
    expect(screen.getByText("TN")).toBeInTheDocument();
  });

  it("uploads a new profile avatar and shows a success toast", async () => {
    renderModal("profile");

    const input = screen.getByLabelText("Tải ảnh đại diện lên");
    const file = new File(["avatar"], "avatar.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(mockUploadImage).toHaveBeenCalledWith(file));
    expect(userStoreState.updateProfile).toHaveBeenCalledWith({
      avatar_url: "https://cdn.example.com/avatar.png",
    });
    await waitFor(() =>
      expect(toastApi.success).toHaveBeenCalledWith(
        "Đã cập nhật ảnh đại diện!",
      ),
    );
  });

  // -- Persona agent avatar upload --

  it("uploads a new agent avatar and shows a success toast", async () => {
    renderModal("persona");

    const input = await screen.findByLabelText("Tải ảnh đại diện Agent lên");
    const file = new File(["agent"], "agent.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(mockUploadImage).toHaveBeenCalledWith(file));
    expect(userStoreState.updateSettings).toHaveBeenCalledWith({
      agent_avatar_url: "https://cdn.example.com/avatar.png",
    });
    await waitFor(() =>
      expect(toastApi.success).toHaveBeenCalledWith(
        "Đã cập nhật ảnh đại diện Agent!",
      ),
    );
  });

  // -- MCP Servers tab --

  it("shows the empty state when there are no MCP servers", () => {
    renderModal("mcp");
    expect(
      screen.getByText("Chưa có MCP server nào được kết nối."),
    ).toBeInTheDocument();
  });

  it("renders a connected MCP server and toggles it off", async () => {
    userStoreState.mcpServers = [mcpServer];
    renderModal("mcp");

    expect(screen.getByText("Notion")).toBeInTheDocument();
    expect(
      screen.getByText("https://notion.example.com/sse"),
    ).toBeInTheDocument();

    const toggle = screen.getByRole("switch", {
      name: "Tắt MCP server Notion",
    });
    await userEvent.click(toggle);

    expect(userStoreState.updateMcpServer).toHaveBeenCalledWith("mcp-1", {
      enabled: false,
    });
  });

  it("adds a new MCP server via the form", async () => {
    renderModal("mcp");
    const user = userEvent.setup();

    await user.type(
      screen.getByPlaceholderText("Ví dụ: Notion, Github MCP..."),
      "My Server",
    );
    await user.type(
      screen.getByPlaceholderText("https://example.com/sse"),
      "https://my-server.example.com/sse",
    );
    await user.click(screen.getByRole("button", { name: "Thêm server" }));

    await waitFor(() =>
      expect(userStoreState.createMcpServer).toHaveBeenCalledWith({
        name: "My Server",
        url: "https://my-server.example.com/sse",
      }),
    );
  });

  it("deletes an MCP server after confirming the dialog", async () => {
    userStoreState.mcpServers = [mcpServer];
    renderModal("mcp");
    const user = userEvent.setup();

    await user.click(
      screen.getByRole("button", { name: "Xoá MCP server Notion" }),
    );

    expect(await screen.findByText("Xoá MCP server?")).toBeInTheDocument();
    expect(
      screen.getByText('MCP server "Notion" sẽ bị xoá vĩnh viễn.'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Xoá" }));

    await waitFor(() =>
      expect(userStoreState.deleteMcpServer).toHaveBeenCalledWith("mcp-1"),
    );
  });

  // -- Skills tab --

  it("toggles a builtin skill off", async () => {
    renderModal("skills");

    const toggle = screen.getByRole("switch", {
      name: "Tắt skill api-designer",
    });
    await userEvent.click(toggle);

    expect(userStoreState.toggleBuiltinSkill).toHaveBeenCalledWith(
      "api-designer",
      false,
    );
  });

  it("shows a builtin skill as disabled when it is in disabledBuiltinSkills", () => {
    userStoreState.disabledBuiltinSkills = ["api-designer"];
    renderModal("skills");

    expect(
      screen.getByRole("switch", { name: "Bật skill api-designer" }),
    ).toBeInTheDocument();
  });

  it("shows the empty state when there are no custom skills", () => {
    renderModal("skills");
    expect(
      screen.getByText("Bạn chưa tạo skill tự tạo nào."),
    ).toBeInTheDocument();
  });

  it("renders a custom skill and deletes it after confirming the dialog", async () => {
    userStoreState.skills = [customSkill];
    renderModal("skills");
    const user = userEvent.setup();

    expect(screen.getByText("weekly-report")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Xoá skill weekly-report" }),
    );

    expect(await screen.findByText("Xoá skill?")).toBeInTheDocument();
    expect(
      screen.getByText('Skill "weekly-report" sẽ bị xoá vĩnh viễn.'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Xoá" }));

    await waitFor(() =>
      expect(userStoreState.deleteSkill).toHaveBeenCalledWith("skill-1"),
    );
  });

  it("adds a new custom skill via the form with comma-separated triggers", async () => {
    renderModal("skills");
    const user = userEvent.setup();

    await user.type(
      screen.getByPlaceholderText("Ví dụ: weekly-report"),
      "standup-notes",
    );
    await user.type(
      screen.getByPlaceholderText(
        "Hướng dẫn chi tiết cho J.A.R.V.I.S. khi kích hoạt skill này...",
      ),
      "Nội dung skill chi tiết",
    );
    await user.type(
      screen.getByPlaceholderText("vd: báo cáo, weekly report, tổng kết tuần"),
      "standup, daily notes",
    );
    await user.click(screen.getByRole("button", { name: "Tạo skill" }));

    await waitFor(() =>
      expect(userStoreState.createSkill).toHaveBeenCalledWith({
        name: "standup-notes",
        description: "",
        when_to_use: "",
        content: "Nội dung skill chi tiết",
        triggers: ["standup", "daily notes"],
      }),
    );
  });

  // -- i18n wiring sanity check --

  it("renders English labels for the new MCP and Skills tabs", async () => {
    await i18n.changeLanguage("en");
    renderModal("mcp");

    expect(
      screen.getByText(
        "Connect your own MCP (Model Context Protocol) servers to extend J.A.R.V.I.S.'s toolset",
      ),
    ).toBeInTheDocument();
  });
});
