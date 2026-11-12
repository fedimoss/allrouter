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
import { Button, Modal, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers';
import './registration-captcha.css';

export default function RegistrationCaptcha({
  visible,
  email,
  onVerified,
  onCancel,
}) {
  const { t } = useTranslation();
  const [challenge, setChallenge] = useState(null);
  const [clicks, setClicks] = useState([]);
  const [loading, setLoading] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [imageReady, setImageReady] = useState(false);
  const [invalid, setInvalid] = useState(false);
  const [error, setError] = useState('');
  const [expiresAt, setExpiresAt] = useState(0);
  const [secondsLeft, setSecondsLeft] = useState(0);
  const [keyboardCursor, setKeyboardCursor] = useState(null);
  const startedAtRef = useRef(0);
  const requestRef = useRef(null);
  const generationRef = useRef(0);

  const loadChallenge = useCallback(async () => {
    requestRef.current?.abort();
    const controller = new AbortController();
    requestRef.current = controller;
    const generation = ++generationRef.current;
    setLoading(true);
    setVerifying(false);
    setChallenge(null);
    setClicks([]);
    setImageReady(false);
    setInvalid(false);
    setError('');
    setKeyboardCursor(null);
    try {
      const res = await API.post(
        '/api/user/register/captcha',
        { email },
        {
          signal: controller.signal,
          skipErrorHandler: true,
          withCredentials: true,
        },
      );
      if (generation !== generationRef.current) return;
      if (!res.data.success) {
        setError(
          res.data.message ||
            t('Unable to load human verification. Please try again.'),
        );
        return;
      }
      setChallenge(res.data.data);
      const ttl = res.data.data.expires_in;
      setExpiresAt(Date.now() + ttl * 1000);
      setSecondsLeft(ttl);
    } catch (err) {
      if (controller.signal.aborted || generation !== generationRef.current) {
        return;
      }
      setError(
        err.response?.data?.message ||
          t('Unable to load human verification. Please try again.'),
      );
    } finally {
      if (generation === generationRef.current) setLoading(false);
    }
  }, [email, t]);

  useEffect(() => {
    if (visible) loadChallenge();
    return () => {
      generationRef.current++;
      requestRef.current?.abort();
    };
  }, [visible, loadChallenge]);

  useEffect(() => {
    if (!visible || !challenge) return;
    const update = () => {
      setSecondsLeft(Math.max(0, Math.ceil((expiresAt - Date.now()) / 1000)));
    };
    const timer = setInterval(update, 1000);
    update();
    return () => clearInterval(timer);
  }, [visible, challenge, expiresAt]);

  const expired = Boolean(challenge) && secondsLeft === 0;
  const selectable =
    challenge && imageReady && !loading && !verifying && !invalid && !expired;

  const addClick = (x, y) => {
    if (!selectable || clicks.length >= challenge.required_clicks) return;
    const timeMS = Math.round(performance.now() - startedAtRef.current);
    setClicks((current) => {
      if (current.length >= challenge.required_clicks) return current;
      return [
        ...current,
        {
          x,
          y,
          time_ms: timeMS,
        },
      ];
    });
  };

  const handlePointerDown = (event) => {
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    if (!selectable) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    addClick(
      ((event.clientX - rect.left) / rect.width) * challenge.width,
      ((event.clientY - rect.top) / rect.height) * challenge.height,
    );
  };

  const handleKeyDown = (event) => {
    if (!selectable) return;
    const cursor = keyboardCursor || {
      x: challenge.width / 8,
      y: challenge.height / 8,
    };
    const offsets = {
      ArrowLeft: [event.shiftKey ? -5 : -challenge.width / 4, 0],
      ArrowRight: [event.shiftKey ? 5 : challenge.width / 4, 0],
      ArrowUp: [0, event.shiftKey ? -5 : -challenge.height / 4],
      ArrowDown: [0, event.shiftKey ? 5 : challenge.height / 4],
    };
    if (offsets[event.key]) {
      event.preventDefault();
      const [dx, dy] = offsets[event.key];
      setKeyboardCursor({
        x: Math.min(challenge.width, Math.max(0, cursor.x + dx)),
        y: Math.min(challenge.height, Math.max(0, cursor.y + dy)),
      });
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      addClick(cursor.x, cursor.y);
    } else if (event.key === 'Backspace') {
      event.preventDefault();
      setClicks((current) => current.slice(0, -1));
    }
  };

  const verify = async () => {
    if (!selectable || clicks.length !== challenge.required_clicks) return;
    const generation = generationRef.current;
    setVerifying(true);
    setError('');
    try {
      const res = await API.post(
        '/api/user/register/captcha/verify',
        { email, challenge_id: challenge.id, clicks },
        {
          signal: requestRef.current.signal,
          skipErrorHandler: true,
          withCredentials: true,
        },
      );
      if (generation !== generationRef.current) return;
      if (res.data.success) {
        onVerified(res.data.data);
      } else {
        setInvalid(true);
        setError(
          res.data.message ||
            t(
              'Human verification failed. Refresh the challenge and try again.',
            ),
        );
      }
    } catch (err) {
      if (generation !== generationRef.current) return;
      setInvalid(true);
      setError(
        err.response?.data?.message ||
          t('Human verification failed. Refresh the challenge and try again.'),
      );
    } finally {
      if (generation === generationRef.current) setVerifying(false);
    }
  };

  return (
    <Modal
      title={t('Human verification')}
      visible={visible}
      onCancel={onCancel}
      maskClosable={false}
      width={440}
      style={{ maxWidth: 'calc(100vw - 24px)' }}
      footer={
        <div className='registration-captcha-footer'>
          <Button onClick={onCancel}>{t('Cancel')}</Button>
          <Button onClick={loadChallenge} disabled={loading || verifying}>
            {t('Refresh challenge')}
          </Button>
          <Button
            type='primary'
            theme='solid'
            onClick={verify}
            loading={verifying}
            disabled={
              !selectable || clicks.length !== challenge?.required_clicks
            }
          >
            {t('Verify and continue')}
          </Button>
        </div>
      }
    >
      <div className='registration-captcha'>
        <p className='registration-captcha-instructions'>
          {t(
            'Click the matching shapes below in the left-to-right order shown above.',
          )}
        </p>
        {loading && (
          <div className='registration-captcha-loading'>
            <Spin size='large' />
            <span>{t('Generating a verification challenge...')}</span>
          </div>
        )}
        {challenge && (
          <>
            <div className='registration-captcha-prompt'>
              <div
                className='registration-captcha-order'
                style={{
                  gridTemplateColumns: `repeat(${challenge.required_clicks}, 1fr)`,
                }}
                aria-hidden='true'
              >
                {Array.from({ length: challenge.required_clicks }, (_, i) => (
                  <span key={i}>{i + 1}</span>
                ))}
              </div>
              <img
                src={challenge.prompt_image}
                alt={t('Match these shapes from left to right')}
                draggable={false}
              />
            </div>
            <div
              className='registration-captcha-board'
              style={{
                aspectRatio: `${challenge.width} / ${challenge.height}`,
              }}
              role='button'
              tabIndex={0}
              aria-label={t(
                'Use arrow keys to move, Enter to select, and Backspace to undo',
              )}
              aria-disabled={!selectable}
              onPointerDown={handlePointerDown}
              onKeyDown={handleKeyDown}
            >
              <img
                src={challenge.image}
                alt={t('Random shapes for human verification')}
                draggable={false}
                onLoad={() => {
                  startedAtRef.current = performance.now();
                  setImageReady(true);
                }}
                onError={() => {
                  setInvalid(true);
                  setError(
                    t('Unable to load human verification. Please try again.'),
                  );
                }}
              />
              {clicks.map((click, i) => (
                <span
                  className='registration-captcha-marker'
                  key={i}
                  style={{
                    left: `${(click.x / challenge.width) * 100}%`,
                    top: `${(click.y / challenge.height) * 100}%`,
                  }}
                  aria-hidden='true'
                >
                  {i + 1}
                </span>
              ))}
              {keyboardCursor && (
                <span
                  className='registration-captcha-cursor'
                  style={{
                    left: `${(keyboardCursor.x / challenge.width) * 100}%`,
                    top: `${(keyboardCursor.y / challenge.height) * 100}%`,
                  }}
                  aria-hidden='true'
                >
                  +
                </span>
              )}
            </div>
            <div className='registration-captcha-progress'>
              <span role='status' aria-live='polite'>
                {t('Selected {{count}} / {{total}} shapes', {
                  count: clicks.length,
                  total: challenge.required_clicks,
                })}
              </span>
              <Button
                size='small'
                theme='borderless'
                onClick={() => setClicks((current) => current.slice(0, -1))}
                disabled={!selectable || clicks.length === 0}
              >
                {t('Undo last selection')}
              </Button>
            </div>
            <p className='registration-captcha-hint'>
              {expired
                ? t('This challenge has expired. Please refresh it.')
                : t('This challenge expires in {{seconds}} seconds', {
                    seconds: secondsLeft,
                  })}
            </p>
            <p className='registration-captcha-hint'>
              {t(
                'Use arrow keys to move, Enter to select, and Backspace to undo',
              )}
            </p>
          </>
        )}
        {error && (
          <div className='registration-captcha-error' role='alert'>
            {error}
          </div>
        )}
      </div>
    </Modal>
  );
}
