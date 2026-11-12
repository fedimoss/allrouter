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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import RegistrationCaptcha from '../../components/auth/RegistrationCaptcha';

export default function useRegistrationCaptcha(email) {
  const normalizedEmail = String(email || '')
    .trim()
    .toLowerCase();
  const [visible, setVisible] = useState(false);
  const grantRef = useRef(null);
  const pendingRef = useRef(null);

  const cancel = useCallback(() => {
    const pending = pendingRef.current;
    pendingRef.current = null;
    pending?.resolve(null);
    setVisible(false);
  }, []);

  useEffect(() => {
    grantRef.current = null;
    cancel();
  }, [normalizedEmail, cancel]);

  useEffect(() => {
    return () => {
      pendingRef.current?.resolve(null);
      pendingRef.current = null;
    };
  }, []);

  const getCaptchaToken = useCallback(
    (purpose) => {
      const grant = grantRef.current;
      const field = purpose === 'email' ? 'email_token' : 'registration_token';
      const expiry =
        purpose === 'email' ? 'emailExpiresAt' : 'registrationExpiresAt';
      if (
        grant?.email === normalizedEmail &&
        grant[field] &&
        Date.now() < grant[expiry]
      ) {
        return Promise.resolve(grant[field]);
      }
      if (pendingRef.current) return Promise.resolve(null);
      const promise = new Promise((resolve) => {
        pendingRef.current = { resolve, purpose, email: normalizedEmail };
      });
      setVisible(true);
      return promise;
    },
    [normalizedEmail],
  );

  const onVerified = useCallback(
    (data) => {
      const pending = pendingRef.current;
      if (!pending || pending.email !== normalizedEmail) return;
      grantRef.current = {
        ...data,
        email: normalizedEmail,
        emailExpiresAt:
          Date.now() + Math.max(0, data.email_expires_in - 2) * 1000,
        registrationExpiresAt:
          Date.now() + Math.max(0, data.registration_expires_in - 2) * 1000,
      };
      pendingRef.current = null;
      const field =
        pending.purpose === 'email' ? 'email_token' : 'registration_token';
      pending.resolve(data[field]);
      setVisible(false);
    },
    [normalizedEmail],
  );

  const consumeEmailToken = useCallback((token) => {
    if (grantRef.current?.email_token === token) {
      grantRef.current.email_token = null;
    }
  }, []);

  const invalidateRegistrationToken = useCallback(() => {
    if (grantRef.current) grantRef.current.registration_token = null;
  }, []);

  return {
    getCaptchaToken,
    consumeEmailToken,
    invalidateRegistrationToken,
    captchaVisible: visible,
    captchaModal: (
      <RegistrationCaptcha
        visible={visible}
        email={normalizedEmail}
        onVerified={onVerified}
        onCancel={cancel}
      />
    ),
  };
}
