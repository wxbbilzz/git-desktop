// 最近打开过的仓库。
//
// 只存在本机浏览器存储里（localStorage），不上传、不进版本库。
// 用途是给标题栏的「切换项目」下拉提供候选列表。
//
// 这和「启动时不自动打开上次的仓库」并不矛盾：这里不替用户做决定，
// 只是把打开过的仓库摆出来，点哪个由用户选。

const STORAGE_KEY = "lazygit-desktop:recent-repos";

/** 最多记这么多条，超出的按时间淘汰。 */
export const MAX_RECENT = 8;

export interface RecentRepo {
  path: string;
  name: string;
  /** 最后一次打开的时间戳，仅用于排序 */
  when: number;
}

/** 取路径最后一段，作为没有仓库名时的兜底。 */
export function basename(path: string): string {
  const parts = path.split("/").filter(Boolean);
  return parts[parts.length - 1] ?? path;
}

/** 路径太长，下拉里只留最后两段，例如 `…/Desktop/git操作`。 */
export function shortPath(path: string): string {
  const parts = path.split("/").filter(Boolean);
  return parts.length <= 2 ? path : "…/" + parts.slice(-2).join("/");
}

/**
 * 读取最近仓库列表。
 *
 * 存储里的内容可能被手改过、也可能是旧版本写的，所以逐条校验，
 * 拿到不合规的就丢弃，而不是让界面崩掉。
 */
export function loadRecentRepos(): RecentRepo[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((item): item is Partial<RecentRepo> => {
        if (!item || typeof item !== "object") return false;
        const path = (item as Partial<RecentRepo>).path;
        return typeof path === "string" && path !== "";
      })
      .map((item) => ({
        path: item.path as string,
        name:
          typeof item.name === "string" && item.name
            ? item.name
            : basename(item.path as string),
        when: typeof item.when === "number" ? item.when : 0,
      }))
      .slice(0, MAX_RECENT);
  } catch {
    // 隐私模式、存储被禁用、JSON 被改坏：一律当作「没有记录」
    return [];
  }
}

export function saveRecentRepos(list: RecentRepo[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(list.slice(0, MAX_RECENT)));
  } catch {
    /* 写不进去不影响使用 */
  }
}

/** 把 path 插到最前面（同路径去重），并裁到上限。 */
export function pushRecentRepo(
  list: RecentRepo[],
  path: string,
  name: string,
): RecentRepo[] {
  return [
    { path, name: name || basename(path), when: Date.now() },
    ...list.filter((r) => r.path !== path),
  ].slice(0, MAX_RECENT);
}

/**
 * 剔除一条记录。
 *
 * 用在「点开却打不开」的场景：目录被删掉或移走了，那条记录再留着
 * 只会让人反复点到同一个错误上。名字叫 forget 而不是 remove，
 * 是因为这里只是忘掉路径，绝不去碰磁盘上的任何东西。
 */
export function forgetRecentRepo(
  list: RecentRepo[],
  path: string,
): RecentRepo[] {
  return list.filter((r) => r.path !== path);
}
