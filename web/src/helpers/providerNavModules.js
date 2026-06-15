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

export const PROVIDER_NAV_AGENT_PARTNER_KEY = 'agent_partner';

export function parseProviderNavModules(navModules) {
  if (!navModules) {
    return {};
  }
  if (typeof navModules === 'object') {
    return navModules;
  }
  try {
    const parsed = JSON.parse(navModules);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

export function stringifyProviderNavModules(config, updates = {}) {
  const modules = parseProviderNavModules(config?.nav_modules);
  return JSON.stringify({
    ...modules,
    ...updates,
  });
}

export function isProviderAgentPartnerEnabled(config) {
  const modules = parseProviderNavModules(config?.nav_modules);
  return modules[PROVIDER_NAV_AGENT_PARTNER_KEY] !== false;
}

export function shouldShowProviderAgentPartner(status) {
  const providerConfig = status?.provider_config;
  if (!providerConfig?.enabled) {
    return true;
  }
  return isProviderAgentPartnerEnabled(providerConfig);
}
