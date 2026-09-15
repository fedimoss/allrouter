/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { history } from './history';
import { API } from './api';
import Loading from '../components/common/ui/Loading';
import {
  getProviderId,
  hasUserPermission,
  isAdmin,
  isProviderOwner,
} from './utils';

export function authHeader() {
  // return authorization header with jwt token
  let user = JSON.parse(localStorage.getItem('user'));

  if (user && user.token) {
    return { Authorization: 'Bearer ' + user.token };
  } else {
    return {};
  }
}

export const AuthRedirect = ({ children }) => {
  const user = localStorage.getItem('user');

  if (user) {
    return <Navigate to='/console' replace />;
  }

  return children;
};

function PrivateRoute({ children }) {
  const location = useLocation();
  if (!localStorage.getItem('user')) {
    return <Navigate to='/login' state={{ from: location }} replace />;
  }
  return children;
}

export function AdminRoute({ children }) {
  const raw = localStorage.getItem('user');
  if (!raw) {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  try {
    const user = JSON.parse(raw);
    if (user && typeof user.role === 'number' && user.role >= 10) {
      return children;
    }
  } catch (e) {
    // ignore
  }
  return <Navigate to='/forbidden' replace />;
}

// 管理页面路由守卫：管理员/超管放行，普通用户需被授予对应模块权限
export function AdminOrPermissionRoute({ module, children }) {
  const raw = localStorage.getItem('user');
  if (!raw) {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  try {
    if (hasUserPermission(module)) {
      return children;
    }
  } catch (e) {
    // ignore
  }
  return <Navigate to='/forbidden' replace />;
}

const readCachedUser = (raw) => {
  if (!raw) return null;
  try {
    const user = JSON.parse(raw);
    return user && typeof user === 'object' ? user : null;
  } catch (e) {
    return null;
  }
};

// The profile endpoint returns the user's database provider_id.  On a
// provider domain that value can differ from the current tenant, so only
// refresh role/ownership/module grants here and deliberately retain the
// tenant fields already established by login.
const syncCachedUserPermissions = (profile) => {
  if (!profile || typeof profile !== 'object') return;
  const current = readCachedUser(localStorage.getItem('user'));
  if (!current) return;
  const next = { ...current };
  ['role', 'status', 'is_provider_owner'].forEach((key) => {
    if (profile[key] !== undefined) next[key] = profile[key];
  });
  // GetSelf currently always includes this key, but keep the presence check
  // so an older server response that omits it does not erase a valid cache.
  // An explicit null/undefined value means that all module grants were
  // revoked and must not fall back to a stale `permissions` value.
  if (Object.prototype.hasOwnProperty.call(profile, 'module_permissions')) {
    next.module_permissions = Array.isArray(profile.module_permissions)
      ? profile.module_permissions
      : [];
  }
  localStorage.setItem('user', JSON.stringify(next));
};

// 服务商控制台路由守卫：服务商属主拥有完整访问权，管理员可访问面向
// 管理员的服务商模块；普通服务商成员仅能进入已授予的模块。ownerOnly
// 用于后端明确限定属主的页面（例如用户管理、服务商选项设置）。
export function ProviderPermissionRoute({
  module,
  ownerOnly = false,
  allowAdmin = true,
  children,
}) {
  const raw = localStorage.getItem('user');
  const cachedUser = readCachedUser(raw);
  // `is_provider_owner` is account-level cache data. A user can be linked to
  // more than one provider, so it cannot establish ownership of the tenant
  // currently selected by the backend/domain context.
  const providerScoped = Number(cachedUser?.provider_id) > 0;
  let cachedAccess = false;
  try {
    const admin = isAdmin();
    if (!providerScoped && isProviderOwner()) {
      cachedAccess = true;
    } else if (!providerScoped && !ownerOnly && allowAdmin && admin) {
      // Admin-facing provider pages are bound to the main-site tenant. A
      // main-site administrator who is not the provider owner must not enter
      // a self-service page while visiting a provider-domain tenant.
      cachedAccess = true;
    } else if (!providerScoped && !ownerOnly && module && !admin) {
      // hasUserPermission intentionally treats administrators as having every
      // module; administrators are handled by the branch above so that
      // allowAdmin=false remains effective.
      cachedAccess = hasUserPermission(module);
    }
  } catch (e) {
    // A malformed/stale local cache is checked against the profile endpoint
    // below rather than being trusted.
  }

  const [verifiedAccess, setVerifiedAccess] = useState(null);

  useEffect(() => {
    let cancelled = false;
    if (!raw) {
      setVerifiedAccess(false);
      return () => {
        cancelled = true;
      };
    }
    // A stale cache can miss a newly granted module.  Refresh the authoritative
    // profile before deciding, so direct URL navigation does not permanently
    // reject a user until they log in again.
    setVerifiedAccess(null);
    // Provider-domain ownership is tenant-scoped. Refresh account grants,
    // then ask tenant-bound endpoint for authoritative owner.
    const profileRequest = API.get('/api/user/self', {
      skipErrorHandler: true,
    });
    profileRequest
      .then((res) => {
        if (cancelled) return;
        const profile = res.data?.success ? res.data.data : null;
        if (!profile) {
          setVerifiedAccess(false);
          return;
        }
        syncCachedUserPermissions(profile);
        if (!providerScoped) {
          let allowed = false;
          try {
            const admin = isAdmin();
            if (isProviderOwner()) {
              allowed = true;
            } else if (
              !ownerOnly &&
              allowAdmin &&
              getProviderId() === 0 &&
              admin
            ) {
              allowed = true;
            } else if (!ownerOnly && module && !admin) {
              allowed = hasUserPermission(module);
            }
          } catch (e) {
            allowed = false;
          }
          setVerifiedAccess(allowed);
          return;
        }

        // Provider endpoint resolves current domain/tenant; owner_user_id is
        // the only ownership signal trusted for provider-scoped routes.
        API.get('/api/provider/self', { skipErrorHandler: true })
          .then((providerRes) => {
            if (cancelled) return;
            const tenant = providerRes.data?.success
              ? providerRes.data.data
              : null;
            if (!tenant) {
              setVerifiedAccess(false);
              return;
            }
            const currentUserId = Number(cachedUser?.id);
            const ownerUserId = Number(tenant.owner_user_id);
            const tenantOwner =
              Number.isSafeInteger(currentUserId) &&
              currentUserId > 0 &&
              Number.isSafeInteger(ownerUserId) &&
              ownerUserId > 0 &&
              currentUserId === ownerUserId;
            if (tenantOwner) {
              setVerifiedAccess(true);
              return;
            }
            let allowed = false;
            try {
              const admin = isAdmin();
              if (!ownerOnly && module && !admin) {
                allowed = hasUserPermission(module);
              }
            } catch (e) {
              allowed = false;
            }
            setVerifiedAccess(allowed);
          })
          .catch(() => {
            if (!cancelled) setVerifiedAccess(false);
          });
      })
      .catch(() => {
        if (!cancelled) setVerifiedAccess(false);
      });

    return () => {
      cancelled = true;
    };
  }, [
    raw,
    cachedAccess,
    providerScoped,
    cachedUser?.id,
    module,
    ownerOnly,
    allowAdmin,
  ]);

  if (!cachedUser) {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  if (verifiedAccess === true) {
    return children;
  }
  if (verifiedAccess === null) {
    return <Loading />;
  }
  return <Navigate to='/forbidden' replace />;
}

export { PrivateRoute };
