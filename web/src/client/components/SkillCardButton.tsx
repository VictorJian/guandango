import React from 'react';
import { SkillCard, SkillCardType, SkillCardNames } from '../../shared/types';

interface Props {
  skill: SkillCard;
  onClick: () => void;
  disabled?: boolean;
}

const skillColors: { [key in SkillCardType]: string } = {
  [SkillCardType.DrawTwo]: 'from-green-600 to-green-800 border-green-400',      // 無中生有 - 綠色
  [SkillCardType.Steal]: 'from-yellow-600 to-yellow-800 border-yellow-400',     // 順手牽羊 - 黃色
  [SkillCardType.Discard]: 'from-red-600 to-red-800 border-red-400',            // 過河拆橋 - 紅色
  [SkillCardType.Skip]: 'from-blue-600 to-blue-800 border-blue-400',            // 樂不思蜀 - 藍色
  [SkillCardType.Harvest]: 'from-amber-500 to-amber-700 border-amber-300',      // 五穀豐登 - 金色
};

const skillIcons: { [key in SkillCardType]: string } = {
  [SkillCardType.DrawTwo]: '+2',
  [SkillCardType.Steal]: '牽',
  [SkillCardType.Discard]: '拆',
  [SkillCardType.Skip]: '跳',
  [SkillCardType.Harvest]: '豐',
};

export const SkillCardButton: React.FC<Props> = ({ skill, onClick, disabled }) => {
  const colorClass = skillColors[skill.type];
  const icon = skillIcons[skill.type];
  const name = SkillCardNames[skill.type];

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`
        w-16 h-20 rounded-lg bg-gradient-to-br ${colorClass}
        border-2 flex flex-col items-center justify-center
        transition-all duration-200 shadow-lg
        ${disabled 
          ? 'opacity-50 cursor-not-allowed' 
          : 'hover:scale-110 hover:shadow-xl cursor-pointer active:scale-95'}
      `}
      title={name}
    >
      <span className="text-2xl font-bold text-white drop-shadow-md">{icon}</span>
      <span className="text-[10px] text-white/90 mt-1 font-medium">{name}</span>
    </button>
  );
};
