import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import i18n from "@/i18n";
import type { McpServer } from "@/hooks/queries/useMcpServers";
import type { UserSkill } from "@/hooks/queries/useSkills";
import { UserSettingsModal } from "./UserSettingsModal";

// Mock cac store/hook phu thuoc de test tap trung vao phan UI moi gan cho
// avatar upload / MCP servers / skills - khong can goi API that.
vi.mock("@/hooks/queries/useSession", () => ({
  useSession: vi.fn(),
}));
vi.mock("@/hooks/queries/useUserSettings", () => ({
  useUserSettings: vi.fn(),
  useUpdateSettings: vi.fn(),
  useUpdateProfile: vi.fn(),
  useChangePassword: vi.fn(),
}));
vi.mock("@/hooks/queries/useMcpServers", () => ({
  useMcpServers: vi.fn(),
}));
vi.mock("@/hooks/queries/useSkills", () => ({
  useSkills: vi.fn(),
}));
vi.mock("@/design-system/molecules/Toast", () => ({
  useToast: vi.fn(),
}));
vi.mock("@/lib/upload", () => ({
  uploadImage: vi.fn(),
}));

import { useSession } from "@/hooks/queries/useSession";
import {
  useChangePassword,
  useUpdateProfile,
  useUpdateSettings,
  useUserSettings,
} from "@/hooks/queries/useUserSettings";
import { useMcpServers } from "@/hooks/queries/useMcpServers";
import { useSkills } from "@/hooks/queries/useSkills";
import { useToast } from "@/design-system/molecules/Toast";
import { uploadImage } from "@/lib/upload";

const mockUseSession = useSession as unknown as Mock;
const mockUseUserSettings = useUserSettings as unknown as Mock;
const mockUseUpdateSettings = useUpdateSettings as unknown as Mock;
const mockUseUpdateProfile = useUpdateProfile as unknown as Mock;
const mockUseChangePassword = useChangePassword as unknown as Mock;
const mockUseMcpServers = useMcpServers as unknown as Mock;
const mockUseSkills = useSkills as unknown as Mock;
const mockUseToast = useToast as unknown as Mock;
const mockUploadImage = uploadImage as unknown as Mock;

const toastApi = {
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
};

// has_auth: false + khong co field token - dung API contract moi (API khong
// bao gio tra lai token da luu, chi bao co auth hay khong qua co boolean).
const mcpServer: McpServer = {
  id: "mcp-1",
  name: "Notion",
  transport: "sse",
  url: "https://notion.example.com/sse",
  enabled: true,
  has_auth: false,
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
  // Trước đây là fetchSettings() trả về Promise; giờ settings nằm sẵn trong
  // cache TanStack Query nên chỉ là dữ liệu, không còn hàm fetch.
  settings: {
    persona_preset: "default",
    formality: "neutral",
    verbosity: "normal",
    custom_instructions: "",
    agent_avatar_url: null,
  },
  updateSettings: vi.fn().mockResolvedValue(undefined),
  updateProfile: vi.fn().mockResolvedValue({
    id: "u1",
    email: "trinh@x.com",
    name: "Trinh",
    role: "user",
  }),
  changePassword: vi.fn().mockResolvedValue(undefined),
  createMcpServer: vi.fn().mockResolvedValue(undefined),
  updateMcpServer: vi.fn().mockResolvedValue(undefined),
  deleteMcpServer: vi.fn().mockResolvedValue(undefined),
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
    // Mỗi hook đọc từ cùng một object userStoreState nên test vẫn chỉnh dữ
    // liệu bằng cách gán trực tiếp (vd userStoreState.mcpServers = [...]).
    mockUseSession.mockReturnValue({
      user: {
        id: "u1",
        email: "trinh@x.com",
        name: "Trinh Nguyen",
        role: "user",
      },
      isPending: false,
    });
    mockUseUserSettings.mockImplementation(() => ({
      settings: userStoreState.settings,
      isPending: false,
    }));
    mockUseUpdateSettings.mockImplementation(() => ({
      mutateAsync: userStoreState.updateSettings,
      isPending: userStoreState.isSaving,
    }));
    mockUseUpdateProfile.mockImplementation(() => ({
      mutateAsync: userStoreState.updateProfile,
      isPending: userStoreState.isSaving,
    }));
    mockUseChangePassword.mockImplementation(() => ({
      mutateAsync: userStoreState.changePassword,
      isPending: userStoreState.isSaving,
    }));
    mockUseMcpServers.mockImplementation(() => ({
      mcpServers: userStoreState.mcpServers,
      isLoadingMcp: userStoreState.isLoadingMcp,
      createMcpServer: userStoreState.createMcpServer,
      updateMcpServer: userStoreState.updateMcpServer,
      deleteMcpServer: userStoreState.deleteMcpServer,
    }));
    mockUseSkills.mockImplementation(() => ({
      skills: userStoreState.skills,
      disabledBuiltinSkills: userStoreState.disabledBuiltinSkills,
      isLoadingSkills: userStoreState.isLoadingSkills,
      createSkill: userStoreState.createSkill,
      updateSkill: userStoreState.updateSkill,
      deleteSkill: userStoreState.deleteSkill,
      toggleBuiltinSkill: userStoreState.toggleBuiltinSkill,
    }));
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

  it("adds a new MCP server via the form with the default Streamable HTTP transport", async () => {
    renderModal("mcp");
    const user = userEvent.setup();

    await user.type(
      screen.getByPlaceholderText("Ví dụ: Notion, Github MCP..."),
      "My Server",
    );
    await user.type(
      screen.getByPlaceholderText("https://example.com/mcp"),
      "https://my-server.example.com/mcp",
    );
    await user.click(screen.getByRole("button", { name: "Thêm server" }));

    // Transport mac dinh la "http" (Streamable HTTP) va khong gui auth_token
    // vi user khong nhap token nao.
    await waitFor(() =>
      expect(userStoreState.createMcpServer).toHaveBeenCalledWith({
        name: "My Server",
        transport: "http",
        url: "https://my-server.example.com/mcp",
      }),
    );
  });

  it("adds a new MCP server with SSE transport and an auth token", async () => {
    renderModal("mcp");
    const user = userEvent.setup();

    await user.type(
      screen.getByPlaceholderText("Ví dụ: Notion, Github MCP..."),
      "Legacy Server",
    );
    await user.selectOptions(
      screen.getByLabelText("Giao thức kết nối (Transport)"),
      "sse",
    );
    await user.type(
      screen.getByPlaceholderText("https://example.com/mcp"),
      "https://legacy.example.com/sse",
    );
    await user.type(
      screen.getByPlaceholderText("Dán access token của server MCP..."),
      "secret-token-123",
    );
    await user.click(screen.getByRole("button", { name: "Thêm server" }));

    await waitFor(() =>
      expect(userStoreState.createMcpServer).toHaveBeenCalledWith({
        name: "Legacy Server",
        transport: "sse",
        url: "https://legacy.example.com/sse",
        auth_token: "secret-token-123",
      }),
    );
  });

  it('shows a "Có auth" badge only for servers that have a saved token', () => {
    userStoreState.mcpServers = [
      mcpServer,
      { ...mcpServer, id: "mcp-2", name: "GitHub", has_auth: true },
    ];
    renderModal("mcp");

    // Chi 1 server (GitHub) co has_auth true nen chi co 1 badge "Có auth".
    expect(screen.getAllByText("Có auth")).toHaveLength(1);
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
    // Trang thai tat phai nhin ra ngay qua badge, khong chi dua vao mau toggle.
    expect(screen.getByText("Đã tắt")).toBeInTheDocument();
  });

  it("filters builtin skills by name via the search input", async () => {
    renderModal("skills");
    const user = userEvent.setup();

    expect(screen.getByText("api-designer")).toBeInTheDocument();

    await user.type(
      screen.getByPlaceholderText("Tìm skill theo tên..."),
      "code-review",
    );

    expect(screen.getByText("code-review")).toBeInTheDocument();
    expect(screen.queryByText("api-designer")).not.toBeInTheDocument();
  });

  it('shows more builtin skills after clicking "Xem tất cả"', async () => {
    renderModal("skills");
    const user = userEvent.setup();

    // "debug" la skill thu 9 (ngoai 8 skill hien mac dinh) - phai an luc dau.
    expect(screen.queryByText("debug")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Xem tất cả" }));

    expect(screen.getByText("debug")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thu gọn" })).toBeInTheDocument();
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

  it('opens and closes the "Tạo skill mới" form via the toggle button', async () => {
    renderModal("skills");
    const user = userEvent.setup();

    // Form thu gon mac dinh - khong co field nao cua form hien dien.
    expect(
      screen.queryByPlaceholderText("Ví dụ: weekly-report"),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "+ Tạo skill" }));

    expect(
      screen.getByPlaceholderText("Ví dụ: weekly-report"),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Đóng form tạo skill" }),
    );

    expect(
      screen.queryByPlaceholderText("Ví dụ: weekly-report"),
    ).not.toBeInTheDocument();
  });

  it("adds a new custom skill via the form with comma-separated triggers", async () => {
    renderModal("skills");
    const user = userEvent.setup();

    // Form thu gon mac dinh nen phai bam "+ Tạo skill" truoc khi thao tac.
    await user.click(screen.getByRole("button", { name: "+ Tạo skill" }));

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
