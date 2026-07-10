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

import React from 'react';
import MarkdownRenderer from '../../components/common/markdown/MarkdownRenderer';
import serviceClauseContent from './serviceClause.md?raw';

const ServiceClause = () => {
  return (
    <main className='min-h-screen bg-slate-50 px-4 py-6 text-slate-900 dark:bg-slate-950 dark:text-slate-100 sm:px-6 sm:py-10'>
      <article className='mx-auto w-full max-w-4xl rounded-lg border border-slate-200 bg-white px-5 py-7 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:px-10 sm:py-10 lg:px-14'>
        <MarkdownRenderer
          content={serviceClauseContent}
          fontSize={16}
          className='leading-8'
        />
      </article>
    </main>
  );
};

export default ServiceClause;
