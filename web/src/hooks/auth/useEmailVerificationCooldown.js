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

import { useCallback, useEffect, useState } from 'react';

export default function useEmailVerificationCooldown() {
  const [deadline, setDeadline] = useState(0);
  const [countdown, setCountdown] = useState(0);

  const startCooldown = useCallback((seconds = 60) => {
    const duration = Math.max(1, Math.ceil(Number(seconds) || 60));
    setDeadline(Date.now() + duration * 1000);
    setCountdown(duration);
  }, []);

  useEffect(() => {
    if (!deadline) return;
    const update = () => {
      const remaining = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      setCountdown(remaining);
      if (remaining === 0) setDeadline(0);
    };
    const timer = setInterval(update, 1000);
    update();
    return () => clearInterval(timer);
  }, [deadline]);

  const handleCooldownError = useCallback(
    (error) => {
      if (error.response?.status === 429) {
        startCooldown(
          error.response.data?.retry_after ||
            error.response.headers?.['retry-after'] ||
            60,
        );
      }
    },
    [startCooldown],
  );

  return {
    countdown,
    isCoolingDown: countdown > 0,
    startCooldown,
    handleCooldownError,
  };
}
