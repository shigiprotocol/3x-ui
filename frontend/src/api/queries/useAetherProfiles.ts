import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

export interface AetherProfileOption {
  id: string;
  name: string;
}

interface AetherStatusResponse {
  profiles?: { id: string; name: string }[];
}

async function fetchProfiles(): Promise<AetherProfileOption[]> {
  const msg = await HttpUtil.get<AetherStatusResponse>('/panel/api/aether/status', undefined, {
    silent: true,
  });
  if (!msg?.success || !msg.obj?.profiles) return [];
  return msg.obj.profiles.map((p) => ({ id: p.id, name: p.name }));
}

// Lightweight list of Aether profiles for the inbound form's egress selector.
export function useAetherProfiles() {
  const query = useQuery({
    queryKey: keys.aether.status(),
    queryFn: fetchProfiles,
    retry: false,
    staleTime: 30 * 1000,
  });
  return query.data ?? [];
}
