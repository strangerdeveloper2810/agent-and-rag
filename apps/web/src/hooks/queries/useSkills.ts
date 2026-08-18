/**
 * Skills của user — /api/user/skills.
 *
 * Một endpoint trả về HAI thứ: custom skill do user tạo, và danh sách builtin
 * skill đã bị tắt. Giữ nguyên trong một queryKey vì server trả cùng một
 * response — tách ra sẽ thành hai request cho cùng dữ liệu.
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import api from "@/lib/http";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";

export interface UserSkill {
  id: string;
  name: string;
  description: string;
  when_to_use: string;
  content: string;
  triggers: string[];
  enabled: boolean;
}

export interface SkillListResult {
  customSkills: UserSkill[];
  disabledBuiltinSkills: string[];
}

export interface CreateSkillInput {
  name: string;
  description?: string;
  when_to_use?: string;
  content: string;
  triggers?: string[];
}

export interface UpdateSkillInput {
  name?: string;
  description?: string;
  when_to_use?: string;
  content?: string;
  triggers?: string[];
  enabled?: boolean;
}

const fetchSkills = (): Promise<SkillListResult> =>
  api.get<SkillListResult>("/api/user/skills");

export const useSkills = ({ enabled = true }: { enabled?: boolean } = {}) => {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: queryKeys.skills(),
    queryFn: fetchSkills,
    staleTime: STALE_TIME.userResources,
    enabled,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.skills() }),
    [queryClient],
  );

  const createMutation = useMutation({
    mutationFn: (data: CreateSkillInput) =>
      api.post<{ skill: UserSkill }>("/api/user/skills", data),
    onSuccess: invalidate,
  });

  const updateMutation = useMutation({
    mutationFn: (vars: { id: string; data: UpdateSkillInput }) =>
      api.patch<{ skill: UserSkill }>(`/api/user/skills/${vars.id}`, vars.data),
    onSuccess: invalidate,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del(`/api/user/skills/${id}`),
    onSuccess: invalidate,
  });

  /**
   * Bật/tắt một builtin skill. Cập nhật lạc quan (optimistic) vì đây là một
   * cái toggle: chờ round-trip mới đổi trạng thái thì công tắc nhìn như bị
   * treo. Lỗi thì trả lại giá trị cũ.
   */
  const toggleMutation = useMutation({
    mutationFn: (vars: { name: string; enabled: boolean }) =>
      api.post(`/api/user/skills/${vars.name}/toggle`, {
        enabled: vars.enabled,
      }),
    onMutate: async ({ name, enabled: nextEnabled }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.skills() });
      const previous = queryClient.getQueryData<SkillListResult>(
        queryKeys.skills(),
      );
      if (previous) {
        const disabled = nextEnabled
          ? previous.disabledBuiltinSkills.filter((n) => n !== name)
          : previous.disabledBuiltinSkills.includes(name)
            ? previous.disabledBuiltinSkills
            : [...previous.disabledBuiltinSkills, name];
        queryClient.setQueryData<SkillListResult>(queryKeys.skills(), {
          ...previous,
          disabledBuiltinSkills: disabled,
        });
      }
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.skills(), context.previous);
      }
    },
    onSettled: invalidate,
  });

  return {
    skills: query.data?.customSkills ?? [],
    disabledBuiltinSkills: query.data?.disabledBuiltinSkills ?? [],
    isLoadingSkills: query.isPending && enabled,
    createSkill: useCallback(
      (data: CreateSkillInput) => createMutation.mutateAsync(data),
      [createMutation],
    ),
    updateSkill: useCallback(
      (id: string, data: UpdateSkillInput) =>
        updateMutation.mutateAsync({ id, data }),
      [updateMutation],
    ),
    deleteSkill: useCallback(
      (id: string) => deleteMutation.mutateAsync(id),
      [deleteMutation],
    ),
    toggleBuiltinSkill: useCallback(
      (name: string, enabled: boolean) =>
        toggleMutation.mutateAsync({ name, enabled }),
      [toggleMutation],
    ),
  };
};
